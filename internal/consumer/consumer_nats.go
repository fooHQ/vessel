//go:build transport_nats || !transport_azqueue

package consumer

import (
	"context"
	"errors"
	"iter"

	proto "github.com/foohq/foojank-proto/go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/foohq/vessel/internal/message"
)

type Arguments struct {
	Connection jetstream.JetStream
	Stream     string
	Consumer   string
}

type Consumer struct {
	args Arguments
}

func New(args Arguments) *Consumer {
	return &Consumer{
		args: args,
	}
}

func (s *Consumer) Messages(ctx context.Context) iter.Seq2[message.Msg, error] {
	consumer, err := s.args.Connection.Consumer(ctx, s.args.Stream, s.args.Consumer)
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

			yield(Message{
				msg:  msg,
				data: data,
			}, nil)
		}
	}
}

type Message struct {
	msg  jetstream.Msg
	data any
}

func (m Message) ID() string {
	return m.msg.Headers().Get(nats.MsgIdHdr)
}

func (m Message) Subject() string {
	return m.msg.Subject()
}

func (m Message) Data() any {
	return m.data
}

func (m Message) Ack() error {
	return m.msg.Ack()
}
