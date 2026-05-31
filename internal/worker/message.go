package worker

import (
	"github.com/foohq/vessel/internal/message"
)

const (
	EventOutput  = "WORKER.EVENTS.OUTPUT"
	EventStopped = "WORKER.EVENTS.STOPPED"
)

type EventWorkerOutput struct {
	WorkerID   string
	OutputData []byte
}

func (e EventWorkerOutput) ID() string {
	return ""
}

func (e EventWorkerOutput) Subject() string {
	return EventOutput
}

func (e EventWorkerOutput) Data() any {
	return e
}

func (e EventWorkerOutput) Ack() error {
	return message.ErrUnsupported
}

type EventWorkerStopped struct {
	WorkerID string
	Status   int64
	Error    error
}

func (e EventWorkerStopped) ID() string {
	return ""
}

func (e EventWorkerStopped) Subject() string {
	return EventStopped
}

func (e EventWorkerStopped) Data() any {
	return e
}

func (e EventWorkerStopped) Ack() error {
	return message.ErrUnsupported
}
