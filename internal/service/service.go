package service

import (
	"context"
	"errors"
	"iter"
	"sync"
	"time"

	protoagent "github.com/foohq/foojank-proto/go/agent"

	"github.com/foohq/vessel/internal/command"
	"github.com/foohq/vessel/internal/message"
	"github.com/foohq/vessel/internal/workmanager"
	"github.com/foohq/vessel/log"
)

type Arguments struct {
	Consumer    Consumer
	Publisher   Publisher
	ConnChecker ConnChecker
	HostInfo    HostInfo
	Commands    command.Registry
}

type Service struct {
	args Arguments
}

func New(args Arguments) *Service {
	return &Service{
		args: args,
	}
}

func (s *Service) Start(ctx context.Context) error {
	log.Debug("Service started", "service", "vessel")
	defer log.Debug("Service stopped", "service", "vessel")

	consumerOutCh := make(chan message.Msg)
	publisherInCh := make(chan message.Msg, 128)
	// Set capacity to the total number of goroutines tracked by the WaitGroup.
	termCh := make(chan struct{}, 4)

	var wg sync.WaitGroup
	var cancels []context.CancelFunc

	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	cancels = append(cancels, consumerCancel)

	wg.Go(func() {
		err := consumer(consumerCtx, s.args.Consumer, consumerOutCh)
		if err != nil {
			log.Debug("Consumer error", "error", err)
		}
		termCh <- struct{}{}
	})

	workManagerCtx, workManagerCancel := context.WithCancel(context.Background())
	cancels = append(cancels, workManagerCancel)

	wg.Go(func() {
		err := workmanager.New(workmanager.Arguments{
			InputCh:  consumerOutCh,
			OutputCh: publisherInCh,
			Commands: s.args.Commands,
		}).Start(workManagerCtx)
		if err != nil {
			log.Debug("WorkManager error", "error", err)
		}
		termCh <- struct{}{}
	})

	beaconCtx, beaconCancel := context.WithCancel(context.Background())
	cancels = append(cancels, beaconCancel)

	wg.Go(func() {
		err := beacon(beaconCtx, s.args.ConnChecker, protoagent.EvtAgentInfoSubject("%s", "%s"), s.args.HostInfo, publisherInCh)
		if err != nil {
			log.Debug("Beacon error", "error", err)
		}
		termCh <- struct{}{}
	})

	publisherCtx, publisherCancel := context.WithCancel(context.Background())
	cancels = append(cancels, publisherCancel)

	wg.Go(func() {
		err := publisher(publisherCtx, s.args.Publisher, publisherInCh)
		if err != nil {
			log.Debug("Publisher error", "error", err)
		}
		termCh <- struct{}{}
	})

	select {
	case <-ctx.Done():
		for _, cancel := range cancels {
			cancel()
			<-termCh
		}
	case <-termCh:
		// If an error occurs in one of the services, cancel all services without waiting for them to finish.
		// Some messages may be lost in the process.
		for _, cancel := range cancels {
			cancel()
		}
	}

	wg.Wait()

	return nil
}

type Consumer interface {
	Messages(context.Context) iter.Seq2[message.Msg, error]
}

func consumer(ctx context.Context, consumer Consumer, outputCh chan message.Msg) error {
	log.Debug("Service started", "service", "vessel.consumer")
	defer log.Debug("Service stopped", "service", "vessel.consumer")

	for msg, err := range consumer.Messages(ctx) {
		if err != nil {
			log.Debug("Cannot read a message", "error", err)
			return err
		}

		err = forwardMessage(outputCh, msg)
		if err != nil {
			log.Debug("Cannot forward a message", "error", err)
			continue
		}
	}

	return nil
}

// TODO: messages should be aggregated so that CreateWorkerRequest can be canceled by StopWorkerRequest.

type Publisher interface {
	Publish(context.Context, message.Msg) error
}

func publisher(ctx context.Context, publisher Publisher, inputCh <-chan message.Msg) error {
	log.Debug("Service started", "service", "vessel.publisher")
	defer log.Debug("Service stopped", "service", "vessel.publisher")

	var exit bool
	var cancel context.CancelFunc

loop:
	for {
		select {
		case msg := <-inputCh:
			err := publisher.Publish(ctx, msg)
			if err != nil {
				log.Debug("Cannot publish a message", "subject", msg.Subject(), "error", err)
				continue
			}

			_ = msg.Ack()

		case <-ctx.Done():
			if !exit {
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				exit = true
				continue loop
			}
			break loop
		}
	}

	cancel()

	if len(inputCh) != 0 {
		log.Debug("Some messages were lost", "count", len(inputCh))
	}

	return nil
}

var _ message.Msg = beaconMessage{}

type beaconMessage struct {
	subject string
	data    any
}

func (m beaconMessage) ID() string {
	return ""
}

func (m beaconMessage) Ack() error {
	return message.ErrUnsupported
}

func (m beaconMessage) Subject() string {
	return m.subject
}

func (m beaconMessage) Data() any {
	return m.data
}

type ConnChecker interface {
	Status() Status
}

func beacon(ctx context.Context, checker ConnChecker, subject string, info HostInfo, outputCh chan<- message.Msg) error {
	log.Debug("Service started", "service", "vessel.beacon")
	defer log.Debug("Service stopped", "service", "vessel.beacon")

	lastStatus := checker.Status()
	triggerCh := make(chan struct{}, 2)
	triggerCh <- struct{}{}

	for {
		select {
		case <-time.After(5 * time.Second):
			status := checker.Status()
			if status == lastStatus {
				continue
			}
			lastStatus = status
			triggerCh <- struct{}{}

		case <-triggerCh:
			if checker.Status() != StatusConnected {
				continue
			}

			err := forwardMessage(outputCh, beaconMessage{
				subject: subject,
				data: protoagent.UpdateClientInfo{
					Username: info.Username(),
					Hostname: info.Hostname(),
					System:   info.System(),
					Address:  info.Address(),
				},
			})
			if err != nil {
				log.Debug("Cannot forward a message", "error", err)
				continue
			}

		case <-ctx.Done():
			return nil
		}
	}
}

type HostInfo interface {
	Username() string
	Hostname() string
	System() string
	Address() string
}

type Status int

const (
	StatusDisconnected Status = iota
	StatusConnected
)

func forwardMessage(outputCh chan<- message.Msg, msg message.Msg) error {
	select {
	case outputCh <- msg:
		return nil
	case <-time.After(10 * time.Second):
		return errors.New("timeout")
	}
}
