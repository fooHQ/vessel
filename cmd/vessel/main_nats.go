//go:build transport_nats || !transport_azqueue

package main

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"iter"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"time"

	proto "github.com/foohq/foojank-proto/go"
	natsfs "github.com/foohq/ren-natsfs"

	vessel "github.com/foohq/vessel/internal"
	execcmd "github.com/foohq/vessel/internal/commands/exec"
	"github.com/foohq/vessel/internal/message"
	"github.com/foohq/vessel/log"

	"github.com/foohq/ren"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/foohq/vessel/internal/commands"
	"github.com/foohq/vessel/internal/dialer"
	"github.com/foohq/vessel/internal/publisher"
)

var (
	AgentID               = ""
	ServerURL             = ""
	ServerCertificate     = ""
	UserJWT               = ""
	UserKey               = ""
	StreamName            = ""
	ConsumerName          = ""
	InboxPrefix           = ""
	ObjectStoreName       = ""
	AwaitMessagesDuration = "" // time.Duration
	IdleDuration          = "" // time.Duration
	IdleJitter            = "" // time.Duration
)

func main() {
	log.Debug("Vessel started")
	defer log.Debug("Vessel stopped")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	connDialer := dialer.New(mustGetAwaitMessagesDuration())
	defer func() {
		_ = connDialer.Close()
	}()

	conn, err := connect(ctx, ServerURL, ServerCertificate, UserJWT, UserKey, connDialer)
	if err != nil {
		log.Debug("Cannot connect to the server", "server", ServerURL, "error", err)
		return
	}
	defer conn.Conn().Close()

	store, err := getObjectStore(ctx, conn, ObjectStoreName)
	if err != nil {
		log.Debug("Cannot obtain object store", "error", err)
		return
	}

	natsFS, err := natsfs.NewFS(ctx, store)
	if err != nil {
		log.Debug("Cannot instantiate nats fs", "error", err)
		return
	}
	defer func() {
		_ = natsFS.Close()
	}()

	cmds := commands.New()
	cmds["exec"] = execcmd.New(map[string]ren.FS{
		"nats": natsFS,
	})

	err = vessel.New(vessel.Arguments{
		ID: AgentID,
		Consumer: NewConsumer(ConsumerConfig{
			Connection: conn,
			Stream:     StreamName,
			Consumer:   ConsumerName,
		}),
		Publisher: publisher.New(publisher.Arguments{
			Connection: conn,
		}),
		ConnChecker: NewConnChecker(ConnCheckerConfig{
			Connection: conn,
		}),
		HostAttrs: vessel.HostAttributes{
			Username: getUsername,
			Hostname: getHostname,
			System:   getSystem,
			Address:  getAddress(conn.Conn()),
		},
		Commands: cmds,
	}).Start(ctx)
	if err != nil {
		log.Debug("Cannot start the agent", "error", err)
		return
	}
}

type ConsumerConfig struct {
	Connection jetstream.JetStream
	Stream     string
	Consumer   string
}

type Consumer struct {
	conf ConsumerConfig
}

func NewConsumer(args ConsumerConfig) *Consumer {
	return &Consumer{
		conf: args,
	}
}

func (s *Consumer) Messages(ctx context.Context) iter.Seq2[message.Msg, error] {
	consumer, err := s.conf.Connection.Consumer(ctx, s.conf.Stream, s.conf.Consumer)
	if err != nil {
		return func(yield func(message.Msg, error) bool) {
			yield(nil, err)
		}
	}

	return func(yield func(message.Msg, error) bool) {
		for ctx.Err() == nil {
			msg, err := consumer.Next(jetstream.FetchContext(ctx))
			if err != nil {
				if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
					yield(nil, err)
					return
				}
				continue
			}

			data, err := proto.Unmarshal(msg.Data())
			if err != nil {
				err := msg.Ack()
				if err != nil {
					yield(nil, err)
					return
				}
				continue
			}

			yield(ConsumerMessage{
				msg:  msg,
				data: data,
			}, nil)
		}
	}
}

type ConsumerMessage struct {
	msg  jetstream.Msg
	data any
}

func (m ConsumerMessage) ID() string {
	return m.msg.Headers().Get(nats.MsgIdHdr)
}

func (m ConsumerMessage) Subject() string {
	return m.msg.Subject()
}

func (m ConsumerMessage) Data() any {
	return m.data
}

func (m ConsumerMessage) Ack() error {
	return m.msg.Ack()
}

type ConnCheckerConfig struct {
	Connection jetstream.JetStream
}

type ConnChecker struct {
	conn jetstream.JetStream
}

func NewConnChecker(conf ConnCheckerConfig) *ConnChecker {
	return &ConnChecker{
		conn: conf.Connection,
	}
}

func (c *ConnChecker) Status() vessel.Status {
	switch c.conn.Conn().Status() {
	case nats.CONNECTED:
		return vessel.StatusConnected
	default:
		return vessel.StatusDisconnected
	}
}

func connect(
	ctx context.Context,
	server,
	serverCertificate,
	userJWT,
	userKey string,
	dialer nats.CustomDialer,
) (jetstream.JetStream, error) {
	opts := []nats.Option{
		nats.CustomInboxPrefix(InboxPrefix),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(mustGetReconnectInterval()),
		nats.ReconnectJitter(mustGetReconnectJitter(), mustGetReconnectJitter()),
		nats.ConnectHandler(connected),
		nats.ReconnectHandler(connected),
		nats.DisconnectErrHandler(disconnected),
		nats.ErrorHandler(failed),
		nats.SetCustomDialer(dialer),
	}

	if userJWT != "" && userKey != "" {
		opts = append(opts, nats.UserJWTAndSeed(userJWT, userKey))
	}

	if serverCertificate != "" {
		opts = append(
			opts,
			nats.TLSHandshakeFirst(),
			nats.ClientTLSConfig(nil, decodeCertificateHandler(serverCertificate)),
		)
	}

	nc, err := nats.Connect(server, opts...)
	if err != nil {
		return nil, err
	}

	for !nc.IsConnected() {
		select {
		case <-time.After(3 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	jetStream, err := jetstream.New(
		nc,
		jetstream.WithDefaultTimeout(10*time.Second),
		jetstream.WithPublishAsyncTimeout(mustGetReconnectInterval()+mustGetReconnectJitter()+(15*time.Second)),
		jetstream.WithPublishAsyncMaxPending(120),
	)
	if err != nil {
		return nil, err
	}

	return jetStream, nil
}

func decodeCertificateHandler(s string) func() (*x509.CertPool, error) {
	return func() (*x509.CertPool, error) {
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(b)
		return pool, nil
	}
}

func getObjectStore(ctx context.Context, conn jetstream.JetStream, store string) (jetstream.ObjectStore, error) {
	return conn.ObjectStore(ctx, store)
}

func mustGetReconnectInterval() time.Duration {
	d, err := time.ParseDuration(IdleDuration)
	if err != nil {
		panic(err)
	}
	return d
}

func mustGetReconnectJitter() time.Duration {
	d, err := time.ParseDuration(IdleJitter)
	if err != nil {
		panic(err)
	}
	return d
}

func mustGetAwaitMessagesDuration() time.Duration {
	d, err := time.ParseDuration(AwaitMessagesDuration)
	if err != nil {
		panic(err)
	}
	return d
}

func getUsername() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.Username
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

func getSystem() string {
	return runtime.GOOS
}

func getAddress(nc *nats.Conn) func() string {
	return func() string {
		ip, err := nc.GetClientIP()
		if err != nil {
			return ""
		}
		return ip.String()
	}
}

func connected(_ *nats.Conn) {
	log.Debug("Connected to the server")
}

func disconnected(_ *nats.Conn, err error) {
	if err != nil {
		log.Debug("Disconnected from the server", "error", err)
	} else {
		log.Debug("Disconnected from the server")
	}
}

func failed(_ *nats.Conn, _ *nats.Subscription, err error) {
	log.Debug("Connection error", "error", err)
}
