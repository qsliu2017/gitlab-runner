package exec

import (
	"context"
	"errors"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type DockerExec struct {
	c      docker.Client
	waiter wait.KillWaiter

	logger logrus.FieldLogger
}

func NewDockerExec(c docker.Client, waiter wait.KillWaiter, logger logrus.FieldLogger) *DockerExec {
	return &DockerExec{
		c:      c,
		waiter: waiter,
		logger: logger,
	}
}

func (e *DockerExec) Exec(ctx context.Context, w io.Writer, r io.Reader, id string) error {
	options := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}

	e.logger.Debugln("Attaching to container", id, "...")
	hijacked, err := e.c.ContainerAttach(ctx, id, options)
	if err != nil {
		return err
	}
	defer hijacked.Close()

	e.logger.Debugln("Starting container", id, "...")
	err = e.c.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	// Copy any output to the build trace
	stdoutErrCh := make(chan error)
	go func() {
		_, errCopy := stdcopy.StdCopy(w, w, hijacked.Reader)
		stdoutErrCh <- errCopy
	}()

	// Write the input to the container and close its STDIN to get it to finish
	stdinErrCh := make(chan error)
	go func() {
		_, errCopy := io.Copy(hijacked.Conn, r)
		_ = hijacked.CloseWrite()
		if errCopy != nil {
			stdinErrCh <- errCopy
		}
	}()

	// Wait until either:
	// - the job is aborted/cancelled/deadline exceeded
	// - stdin has an error
	// - stdout returns an error or nil, indicating the stream has ended and
	//   the container has exited
	select {
	case <-ctx.Done():
		err = errors.New("aborted")
	case err = <-stdinErrCh:
	case err = <-stdoutErrCh:
	}

	if err != nil {
		e.logger.Debugln("Container", id, "finished with", err)
	}

	// Kill and wait for exit.
	// Containers are stopped so that they can be reused by the job.
	return e.waiter.KillWait(ctx, id)
}
