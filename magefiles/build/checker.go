package build

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/samber/lo"
)

const (
	skopeoImage = "quay.io/skopeo/stable:v1.12.0"
)

type ResourceChecker interface {
	Exists() error
}

func NewResourceChecker(c Component) ResourceChecker {
	switch c.Type() {
	case typeDockerImage:
		return newDockerImageChecker(c.Value())
	case typeFile:
		return newFileChecker(c.Value())
	case typeDockerImageArchive:
		return newFileChecker(c.Value())
	}

	return unknownResourceChecker{}
}

type unknownResourceChecker struct {
}

func (unknownResourceChecker) Exists() error {
	return errors.New("unknown")
}

type fileChecker struct {
	file string
}

func newFileChecker(f string) fileChecker {
	return fileChecker{file: f}
}

func (f fileChecker) Exists() error {
	_, err := os.Stat(f.file)
	return err
}

type dockerImageChecker struct {
	image string
}

func newDockerImageChecker(image string) *dockerImageChecker {
	return &dockerImageChecker{image: image}
}

func (d *dockerImageChecker) Exists() error {
	// the results of this function can be cached but there's no need atm
	args := []string{
		"inspect", "--raw", "--no-tags",
		"docker://" + d.image,
	}
	command := "skopeo"
	_, err := exec.LookPath("skopeo")
	if err != nil {
		command = "docker"
		args = append([]string{"run", "--rm", skopeoImage}, args...)
	}

	out, err := exec.Command(command, args...).CombinedOutput()
	if err == nil {
		return nil
	}

	if strings.Contains(string(out), "manifest unknown") {
		return errors.New("manifest unknown")
	}

	// clumsily parse skopeo error message such as
	// time="2023-10-10T22:45:14+03:00" level=fatal msg="Error parsing image name \"docker://gitlab-runner:bleeding\":
	// reading manifest bleeding in docker.io/library/gitlab-runner: requested access to the resource is denied"
	// this isn't pretty, but it's easy, and we don't really care all that much
	msg, _ := lo.Last(strings.Split(string(out), ":"))
	return errors.New(strings.TrimSpace(strings.ReplaceAll(msg, `"`, "")))
}
