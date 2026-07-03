package publisher

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue/v2"
	protoagent "github.com/foohq/foojank-proto/go/agent"
	protoutils "github.com/foohq/foojank-proto/go/utils"

	"github.com/foohq/vessel/internal/message"
)

type QueuePublisherConfig struct {
	AgentID    string
	GatewayID  string
	Connection *azqueue.QueueClient
}

type QueuePublisher struct {
	conf QueuePublisherConfig
}

func NewQueuePublisher(conf QueuePublisherConfig) *QueuePublisher {
	return &QueuePublisher{
		conf: conf,
	}
}

func (p *QueuePublisher) Publish(ctx context.Context, msg message.Msg) error {
	data, err := protoagent.Marshal(protoagent.Envelope{
		Subject: protoutils.FormatString(msg.Subject(), p.conf.GatewayID, p.conf.AgentID),
		Payload: msg.Data(),
	})
	if err != nil {
		return err
	}

	_, err = p.conf.Connection.EnqueueMessage(ctx, string(data), &azqueue.EnqueueMessageOptions{
		TimeToLive:        new(int32(-1)),
		VisibilityTimeout: nil, // TODO
	})
	if err != nil {
		return err
	}

	return nil
}
