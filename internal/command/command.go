package command

import "context"

type Command interface {
	Run(ctx context.Context, args, env []string) (int64, error)
}
