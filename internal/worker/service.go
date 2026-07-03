package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/foohq/vessel/internal/command"
	"github.com/foohq/vessel/internal/message"
	"github.com/foohq/vessel/log"
)

type Arguments struct {
	ID       string
	Command  string
	Args     []string
	Env      []string
	EventCh  chan<- message.Msg
	StdinCh  <-chan []byte
	Commands command.Registry
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
	log.Debug("Service started", "service", "vessel.workmanager.worker", "id", s.args.ID)
	defer log.Debug("Service stopped", "service", "vessel.workmanager.worker", "id", s.args.ID)

	// Capacity must be greater than the total number of goroutines tracked by the WaitGroup.
	termCh := make(chan struct{}, 8)

	var wg sync.WaitGroup
	var cancels []context.CancelFunc

	stdin := command.NewPipe()
	stdinWriterCtx, stdinWriterCancel := context.WithCancel(ctx)
	stdinWriterCancelWrapper := func() {
		_ = stdin.Close()
		stdinWriterCancel()
	}
	cancels = append(cancels, stdinWriterCancelWrapper)

	wg.Go(func() {
		err := stdinWriter(stdinWriterCtx, s.args.StdinCh, stdin)
		if err != nil {
			log.Debug("Stdin writer failed", "error", err)
		}
		termCh <- struct{}{}
	})

	stdout := command.NewPipe()
	stdoutReaderCtx, stdoutReaderCancel := context.WithCancel(ctx)
	stdoutReaderCancelWrapper := func() {
		_ = stdout.Close()
		stdoutReaderCancel()
	}
	cancels = append(cancels, stdoutReaderCancelWrapper)

	wg.Go(func() {
		err := stdoutReader(stdoutReaderCtx, s.args.ID, stdout, s.args.EventCh)
		if err != nil {
			log.Debug("Stdout reader failed", "error", err)
		}
		termCh <- struct{}{}
	})

	runnerCtx, runnerCancel := context.WithCancel(ctx)
	cancels = append(cancels, runnerCancel)

	wg.Go(func() {
		status := s.args.Commands.RunCommand(runnerCtx, s.args.Command, s.args.Args, s.args.Env, stdin, stdout)
		err := forwardMessage(s.args.EventCh, EventWorkerStopped{
			WorkerID: s.args.ID,
			Status:   status.Code(),
			Error:    status.Error(),
		})
		if err != nil {
			log.Debug("Cannot forward a message", "error", err)
		}
		termCh <- struct{}{}
	})

	select {
	case <-ctx.Done():
		for _, cancel := range cancels {
			cancel()
		}
	case <-termCh:
		// If an error occurs in one of the services, cancel all services immediately.
		for _, cancel := range cancels {
			cancel()
		}
	}

	wg.Wait()

	return nil
}

func stdinWriter(ctx context.Context, inputCh <-chan []byte, outputFile command.File) error {
	log.Debug("Service started", "service", "vessel.worker.stdinwriter")
	defer log.Debug("Service stopped", "service", "vessel.worker.stdinwriter")

	for {
		select {
		case data := <-inputCh:
			_, err := outputFile.Write(data)
			if err != nil {
				return nil
			}

		case <-ctx.Done():
			return nil
		}
	}
}

func stdoutReader(ctx context.Context, workerID string, inputFile command.File, outputCh chan<- message.Msg) error {
	log.Debug("Service started", "service", "vessel.worker.stdoutwriter")
	defer log.Debug("Service stopped", "service", "vessel.worker.stdoutwriter")

	for {
		b := make([]byte, 4096)
		n, err := inputFile.Read(b)
		if err != nil {
			return nil
		}

		select {
		case outputCh <- EventWorkerOutput{
			WorkerID:   workerID,
			OutputData: b[:n],
		}:
		case <-ctx.Done():
			return nil
		}
	}
}

func forwardMessage(outputCh chan<- message.Msg, msg message.Msg) error {
	select {
	case outputCh <- msg:
		return nil
	case <-time.After(10 * time.Second):
		return errors.New("timeout")
	}
}
