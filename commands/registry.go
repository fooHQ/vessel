package commands

import (
	"context"
	"errors"

	proto "github.com/foohq/foojank-proto/go"

	"github.com/foohq/vessel/internal/command"
)

type Registry struct {
	commands map[string]Command
}

func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

func (r *Registry) Add(name string, cmd Command) {
	r.commands[name] = cmd
}

func (r *Registry) RunCommand(ctx context.Context, cmd string, args, env []string, stdin, stdout command.File) command.Status {
	cc, ok := r.commands[cmd]
	if !ok {
		return Status{
			code: proto.ExitCommandNotFound,
			err:  errors.New("command not found"),
		}
	}

	code, err := cc.Run(ctx, args, env, stdin, stdout)
	return Status{
		code: code,
		err:  err,
	}
}

type Command interface {
	Run(ctx context.Context, args, env []string, stdin, stdout command.File) (int64, error)
}

type Status struct {
	code int64
	err  error
}

func (s Status) Code() int64 {
	return s.code
}

func (s Status) Error() error {
	return s.err
}
