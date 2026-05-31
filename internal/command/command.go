package command

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"time"
)

type Registry interface {
	RunCommand(ctx context.Context, cmd string, args, env []string, stdin, stdout File) Status
}

type Status interface {
	Code() int64
	Error() error
}

type File interface {
	fs.File
	io.Writer
}

type Pipe struct {
	r *io.PipeReader
	w *io.PipeWriter
}

// NewPipe returns a new Pipe.
func NewPipe() *Pipe {
	r, w := io.Pipe()
	return &Pipe{
		r: r,
		w: w,
	}
}

func (f *Pipe) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if errors.Is(err, io.ErrClosedPipe) {
		return n, fs.ErrClosed
	}
	return n, err
}

func (f *Pipe) Stat() (fs.FileInfo, error) {
	return &pipeInfo{
		name:    "grr",
		size:    0,
		mode:    0,
		modTime: time.Time{},
		isDir:   false,
	}, nil
}

func (f *Pipe) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if errors.Is(err, io.ErrClosedPipe) {
		return n, fs.ErrClosed
	}
	return n, err
}

func (f *Pipe) Close() error {
	wErr := f.w.Close()
	rErr := f.r.Close()

	var err error
	if wErr != nil {
		err = wErr
	}
	if rErr != nil {
		err = rErr
	}
	return err
}

type pipeInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *pipeInfo) Name() string {
	return fi.name
}

func (fi *pipeInfo) Size() int64 {
	return fi.size
}

func (fi *pipeInfo) Mode() fs.FileMode {
	return fi.mode
}

func (fi *pipeInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *pipeInfo) IsDir() bool {
	return fi.isDir
}

func (fi *pipeInfo) Sys() any {
	return nil
}
