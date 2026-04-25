package exec

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/foohq/ren"
	"github.com/foohq/ren/builtins"
	"github.com/foohq/ren/modules"

	proto "github.com/foohq/foojank-proto/go"

	"github.com/foohq/vessel/internal/log"
)

type Command struct {
	stdin       ren.File
	stdout      ren.File
	filesystems map[string]ren.FS
}

func New(stdin, stdout ren.File, filesystems map[string]ren.FS) *Command {
	return &Command{
		stdin:       stdin,
		stdout:      stdout,
		filesystems: filesystems,
	}
}

func (c *Command) Run(ctx context.Context, args, env []string) (int64, error) {
	if len(args) == 0 {
		return proto.ExitFailure, errors.New("missing package path")
	}

	pkg := args[0]
	u, err := url.Parse(pkg)
	if err != nil {
		return proto.ExitFailure, err
	}

	b, err := c.readFile(ctx, u)
	if err != nil {
		return proto.ExitFailure, errors.New("cannot read package '" + u.Path + "': " + err.Error())
	}

	opts := []ren.Option{
		ren.WithArgs(args),
		ren.WithStdin(c.stdin),
		ren.WithStdout(c.stdout),
	}

	for scheme, fs := range c.filesystems {
		opts = append(opts, ren.WithFilesystem(scheme, fs))
	}

	// Configure exit status handler
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	status := proto.ExitSuccess
	opts = append(opts, ren.WithExitHandler(func(code int) {
		log.Debug("on exit", "code", code)
		status = int64(code)
		cancel()
	}))

	// Configure builtins
	for _, builtin := range builtins.Builtins() {
		opts = append(opts, ren.WithBuiltin(builtin))
	}

	// Configure modules
	for _, module := range modules.Modules() {
		opts = append(opts, ren.WithModule(module))
	}

	// Configure environment variables
	// TODO: ren does not support environment variables yet
	/*for i := 0; i < len(env); i += 2 {
		name := env[i]
		value := ""
		if i+1 < len(env) {
			value = env[i+1]
		}
		opts = append(opts, ren.WithEnvVar(name, value))
	}*/

	err = ren.RunBytes(
		ctx,
		b,
		opts...,
	)

	switch {
	case err == nil:
		return status, nil
	case errors.Is(err, context.Canceled):
		return proto.ExitInterrupted, nil
	default:
		return proto.ExitFailure, err
	}
}

func (c *Command) readFile(ctx context.Context, u *url.URL) ([]byte, error) {
	fsType := u.Scheme
	if fsType == "" {
		fsType = "file"
	}

	fs, ok := c.filesystems[fsType]
	if !ok {
		return nil, errors.New("filesystem '" + fsType + "' not found")
	}

	const maxRetries = 5
	var b []byte
	var err error
	for i := 0; i < maxRetries+1; i++ {
		b, err = fs.ReadFile(u.Path)
		if err == nil {
			break
		}

		select {
		case <-time.After(2 * time.Second):
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return b, err
}
