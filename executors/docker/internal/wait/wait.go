package wait

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type Waiter interface {
	Wait(ctx context.Context, containerID string) error
}

type KillWaiter interface {
	Waiter

	KillWait(ctx context.Context, containerID string) error
}

type KillRemoveWaiter interface {
	KillWaiter

	RemoveWait(ctx context.Context, containerID string) error
}

type dockerWaiter struct {
	client docker.Client
}

func NewDockerWaiter(c docker.Client) KillRemoveWaiter {
	return &dockerWaiter{
		client: c,
	}
}

// Wait blocks until the container specified has stopped.
func (d *dockerWaiter) Wait(ctx context.Context, containerID string) error {
	return d.retryWait(ctx, containerID, nil, container.WaitConditionNotRunning)
}

// KillWait blocks (periodically attempting to kill the container) until the
// specified container has stopped.
func (d *dockerWaiter) KillWait(ctx context.Context, containerID string) error {
	return d.retryWait(ctx, containerID, func() {
		_ = d.client.ContainerKill(ctx, containerID, "SIGKILL")
	}, container.WaitConditionNotRunning)
}

// RemoveWait blocks (periodically attempting to remove the container) until the
// specified container has been removed. If the container cannot be found, no
// error is returned, with the assumption that it has previously been removed.
func (d *dockerWaiter) RemoveWait(ctx context.Context, containerID string) error {
	err := d.retryWait(ctx, containerID, func() {
		_ = d.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
	}, container.WaitConditionRemoved)

	if docker.IsErrNotFound(err) {
		return nil
	}

	return err
}

func (d *dockerWaiter) retryWait(
	ctx context.Context,
	containerID string,
	requestFn func(),
	condition container.WaitCondition,
) error {
	retries := 0

	for ctx.Err() == nil {
		err := d.wait(ctx, containerID, requestFn, condition)
		if err == nil {
			return nil
		}

		var e *common.BuildError
		if errors.As(err, &e) || docker.IsErrNotFound(err) || retries > 3 {
			return err
		}
		retries++

		time.Sleep(time.Second)
	}

	return ctx.Err()
}

// wait waits until the container has reached the condition specified.
//
// The passed `requestFn` function is periodically called (to ensure that the
// daemon absolutely receives the request) and is used to stop the container.
func (d *dockerWaiter) wait(
	ctx context.Context,
	containerID string,
	requestFn func(),
	condition container.WaitCondition,
) error {
	statusCh, errCh := d.client.ContainerWait(ctx, containerID, condition)

	if requestFn != nil {
		requestFn()
	}

	for {
		select {
		case <-time.After(time.Second):
			if requestFn != nil {
				requestFn()
			}

		case err := <-errCh:
			return err

		case status := <-statusCh:
			if condition == container.WaitConditionNotRunning && status.StatusCode != 0 {
				return &common.BuildError{
					Inner:    fmt.Errorf("exit code %d", status.StatusCode),
					ExitCode: int(status.StatusCode),
				}
			}

			return nil
		}
	}
}
