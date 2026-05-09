package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/foohq/vessel/internal/commands"
	"github.com/foohq/vessel/internal/log"
	"github.com/foohq/vessel/internal/vessel/message"
)

type Arguments struct {
	ID       string
	Command  string
	Args     []string
	Env      []string
	EventCh  chan<- message.Msg
	StdinCh  <-chan []byte
	Commands commands.Commands
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

	err := forwardMessage(s.args.EventCh, EventWorkerStarted{
		WorkerID: s.args.ID,
	})
	if err != nil {
		log.Debug("Cannot forward a message", "error", err)
		return err
	}

	// Capacity must be greater than the total number of goroutines tracked by the WaitGroup.
	termCh := make(chan struct{}, 8)

	var wg sync.WaitGroup

	stdin := commands.NewPipe()
	stdinWriterCtx, stdinWriterCancel := context.WithCancel(ctx)
	stdinWriterCancelWrapper := func() {
		_ = stdin.Close()
		stdinWriterCancel()
	}
	defer stdinWriterCancelWrapper()

	wg.Go(func() {
		err := stdinWriter(stdinWriterCtx, s.args.StdinCh, stdin)
		if err != nil {
			log.Debug("Stdin writer failed", "error", err)
		}
		termCh <- struct{}{}
	})

	stdout := commands.NewPipe()
	stdoutReaderCtx, stdoutReaderCancel := context.WithCancel(ctx)
	stdoutReaderCancelWrapper := func() {
		_ = stdout.Close()
		stdoutReaderCancel()
	}
	defer stdoutReaderCancelWrapper()

	wg.Go(func() {
		err := stdoutReader(stdoutReaderCtx, s.args.ID, stdout, s.args.EventCh)
		if err != nil {
			log.Debug("Stdout reader failed", "error", err)
		}
		termCh <- struct{}{}
	})

	runnerCtx, runnerCancel := context.WithCancel(ctx)
	defer runnerCancel()

	wg.Go(func() {
		code, err := s.args.Commands.Run(runnerCtx, s.args.Command, s.args.Args, s.args.Env, stdin, stdout)
		if err != nil {
			log.Debug("Runner failed", "error", err)
		}

		err = forwardMessage(s.args.EventCh, EventWorkerStopped{
			WorkerID: s.args.ID,
			Status:   code,
			Error:    err,
		})
		if err != nil {
			log.Debug("Cannot forward a message", "error", err)
		}
		termCh <- struct{}{}
	})

	cancels := []context.CancelFunc{
		runnerCancel,
		stdinWriterCancelWrapper,
		stdoutReaderCancelWrapper,
	}

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

func stdinWriter(ctx context.Context, inputCh <-chan []byte, outputFile commands.File) error {
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

func stdoutReader(ctx context.Context, workerID string, inputFile commands.File, outputCh chan<- message.Msg) error {
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
