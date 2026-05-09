package publisher

import (
	"context"
	"time"

	proto "github.com/foohq/foojank-proto/go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/foohq/vessel/internal/vessel/message"
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
	_, err = p.args.Connection.Publish(
		ctx,
		msg.Subject(),
		data,
		opts...,
	)
	if err != nil {
		return err
	}

	return nil
}
