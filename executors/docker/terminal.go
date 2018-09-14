package docker

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-terminal"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	terminalsession "gitlab.com/gitlab-org/gitlab-runner/session/terminal"
)

func (s *commandExecutor) Connect() (terminalsession.Conn, error) {
	maxRetries := 10
	var counter int

	for s.buildContainer == nil {
		s.Debugln("container might not be created yet, sleeping.")
		time.Sleep(1 * time.Second)
		counter++

		if counter >= maxRetries {
			return nil, errors.New("failed to connect to docker container")
		}
	}

	return terminalConn{
		logger:      &s.BuildLogger,
		client:      s.client,
		containerID: s.buildContainer.ID,
	}, nil
}

type terminalConn struct {
	logger *common.BuildLogger

	client      docker_helpers.Client
	containerID string
}

func (t terminalConn) Start(w http.ResponseWriter, r *http.Request, timeoutCh, disconnectCh chan error) {
	ctx := context.Background()
	execConfig := types.ExecConfig{
		Tty:          true,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          []string{"sh", "-c", "/bin/bash || /bin/sh"},
	}

	exec, err := t.client.ContainerExecCreate(context.Background(), t.containerID, execConfig)
	if err != nil {
		t.logger.Errorln("failed to create exec container for terminal")
	}

	resp, err := t.client.ContainerExecAttach(ctx, exec.ID, execConfig)
	if err != nil {
		t.logger.Errorln("failed to exec attach to container for terminal")
	}

	dockerTTY := newDockerTTY(&resp)

	proxy := terminal.NewDockerProxy(1) // one stopper: terminal exit handler
	terminalsession.ProxyTerminal(
		timeoutCh,
		disconnectCh,
		proxy.StopCh,
		func() {
			terminal.ProxyDocker(w, r, dockerTTY, proxy)
		},
	)
}

func (t terminalConn) Close() error {
	return nil
}

type dockerTTY struct {
	hijackedResp *types.HijackedResponse
}

func (d *dockerTTY) Read(p []byte) (int, error) {
	return d.hijackedResp.Reader.Read(p)
}

func (d *dockerTTY) Write(p []byte) (int, error) {
	return d.hijackedResp.Conn.Write(p)
}

func (d *dockerTTY) Close() error {
	d.hijackedResp.Close()
	return nil
}

func newDockerTTY(hijackedResp *types.HijackedResponse) *dockerTTY {
	return &dockerTTY{
		hijackedResp: hijackedResp,
	}
}
