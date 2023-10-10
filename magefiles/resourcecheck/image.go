package resourcecheck

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/samber/lo"
)

const (
	skopeoImage = "quay.io/skopeo/stable:v1.12.0"
)

type DockerImage struct {
	image string
}

func NewDockerImage(image string) *DockerImage {
	return &DockerImage{image: image}
}

func (d *DockerImage) Exists() error {
	// the results of this function can be cached but there's no need atm
	command := "skopeo inspect --raw --no-tags docker://" + d.image
	_, err := exec.LookPath("skopeo")
	if err != nil {
		command = fmt.Sprintf("docker run --rm %s %s", skopeoImage, command)
	}

	out, err := exec.Command("sh", "-c", command).CombinedOutput()
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
