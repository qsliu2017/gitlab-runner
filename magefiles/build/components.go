package build

import (
	"errors"
	"os"
)

type Component interface {
	Value() string
	Type() string
	Exists() error
}

var ErrUnknownComponentExists = errors.New("unknown")

type component struct {
	value  string
	typ    string
	exists func() error
}

func (c component) Value() string {
	return c.value
}

func (c component) Type() string {
	return c.typ
}

func (c component) Exists() error {
	if c.exists == nil {
		return ErrUnknownComponentExists
	}

	return c.exists()
}

func fileExists(f string) func() error {
	return func() error {
		_, err := os.Stat(f)
		return err
	}
}

func dockerImageExists(img string) func() error {
	return func() error {
		//command := "skopeo inspect --raw --no-tags docker://" + img
		//_, err := exec.LookPath("skopeo")
		//if err != nil {
		//	command = fmt.Sprintf("docker run --rm ")
		//}

		return nil
	}
}

func NewDockerImage(value string) Component {
	return component{
		value: value,
		typ:   "Docker Image",
	}
}

func NewDockerImageArchive(value string) Component {
	return component{
		value:  value,
		typ:    "Docker Image Archive",
		exists: fileExists(value),
	}
}

func NewFile(value string) Component {
	return component{
		value:  value,
		typ:    "File",
		exists: fileExists(value),
	}
}
