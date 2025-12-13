package command

import "context"

const (
	ExitFailure         = 1
	ExitCommandNotFound = 127
	ExitCancelled       = 130
)

type Command interface {
	Run(ctx context.Context, args, env []string) (int, error)
}
