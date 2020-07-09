package user

import (
	"bytes"
	"context"
	"log"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/exec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type Inspect struct {
	c    docker.Client
	exec exec.DockerExec
}

func NewInspect(c docker.Client, exec exec.DockerExec) *Inspect {
	return &Inspect{
		c:    c,
		exec: exec,
	}
}

func (i *Inspect) NotRoot(ctx context.Context, id string) (bool, error) {
	img, _, err := i.c.ImageInspectWithRaw(ctx, id)
	if err != nil {
		return false, err
	}

	if img.Config.User == "" {
		return false, nil
	}

	return true, nil
}

func (i *Inspect) UserID(ctx context.Context, id string) (int, error) {
	b := bytes.NewBuffer([]byte{})
	err := i.exec.Exec(ctx, b, bytes.NewBuffer([]byte("id -u")), id)
	if err != nil {
		return 0, nil
	}

	uID, err := strconv.Atoi(strings.TrimSuffix(b.String(), "\n"))
	if err != nil {
		return 0, err
	}

	return uID, nil
}

func (i *Inspect) GroupID(ctx context.Context, id string) (int, error) {
	b := bytes.NewBuffer([]byte{})
	err := i.exec.Exec(ctx, b, bytes.NewBuffer([]byte("id -g")), id)
	if err != nil {
		return 0, nil
	}

	gID, err := strconv.Atoi(strings.TrimSuffix(b.String(), "\n"))
	if err != nil {
		return 0, err
	}

	log.Printf("gID: %#+v", gID)

	return gID, nil
}
