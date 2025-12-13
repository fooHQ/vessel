package commands

import (
	"context"
	"errors"

	"github.com/foohq/vessel/internal/command"
)

type Commands map[string]command.Command

func New() Commands {
	return make(Commands)
}

func (c Commands) Run(ctx context.Context, cmd string, args, env []string) (int, error) {
	cc, ok := c[cmd]
	if !ok {
		return command.ExitCommandNotFound, errors.New("command not found")
	}
	return cc.Run(ctx, args, env)
}
