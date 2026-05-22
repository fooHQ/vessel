//go:build transport_nats || !transport_azqueue

package publisher

import (
	"context"
	"time"

	proto "github.com/foohq/foojank-proto/go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nuid"

	vessel "github.com/foohq/vessel/internal"
	"github.com/foohq/vessel/internal/message"
)

type Arguments struct {
	Connection jetstream.JetStream
}

type Publisher struct {
	args Arguments
}

func New(args Arguments) *Publisher {
	return &Publisher{
		args: args,
	}
}

func (p *Publisher) Publish(ctx context.Context, msg message.Msg) error {
	data, err := proto.Marshal(msg.Data())
	if err != nil {
		return err
	}

	opts := []jetstream.PublishOpt{
		jetstream.WithRetryAttempts(3),
		jetstream.WithRetryWait(250 * time.Millisecond),
	}
	_, err = p.args.Connection.PublishMsg(
		ctx,
		&nats.Msg{
			Subject: msg.Subject(),
			Header: map[string][]string{
				jetstream.MsgIDHeader: {
					nuid.Next(),
				},
			},
			Data: data,
		},
		opts...,
	)
	if err != nil {
		return err
	}

	return nil
}

func (p *Publisher) Status() vessel.Status {
	switch p.args.Connection.Conn().Status() {
	case nats.CONNECTED:
		return vessel.StatusConnected
	default:
		return vessel.StatusDisconnected
	}
}
