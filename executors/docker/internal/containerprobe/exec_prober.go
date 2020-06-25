package containerprobe

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/probe"
)

var _ probe.Prober = (*ExecProbe)(nil)

type ExecProbe struct {
	Client      docker.Client
	ContainerID string
	Names       []string
	Cmd         []string
}

func (p *ExecProbe) String() string {
	return fmt.Sprintf("container=%s, names=%v, cmd=%v", p.ContainerID, p.Names, p.Cmd)
}

func (p *ExecProbe) Probe(ctx context.Context, timeout time.Duration) error {
	resp, err := p.Client.ContainerExecCreate(ctx, p.ContainerID, types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          p.Cmd,
	})
	if err != nil {
		return err
	}

	body, err := p.exec(ctx, resp.ID, timeout)
	if err != nil {
		return err
	}

	inspect, err := p.Client.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		return err
	}

	if inspect.ExitCode == 0 {
		return nil
	}

	return fmt.Errorf("Exec probe failed, exit code: %d, body: %v", inspect.ExitCode, string(body))
}

func (p *ExecProbe) exec(ctx context.Context, containerID string, timeout time.Duration) ([]byte, error) {
	attachCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	hijacked, err := p.Client.ContainerExecAttach(attachCtx, containerID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	defer hijacked.Close()

	return ioutil.ReadAll(io.LimitReader(hijacked.Reader, 10*1024))
}
