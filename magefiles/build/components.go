package build

import (
	"errors"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/resourcecheck"
)

type Component interface {
	Value() string
	Type() string
	Exists() error
}

var ErrUnknownComponentExists = errors.New("unknown")

type component struct {
	value   string
	typ     string
	checker resourcecheck.ResourceChecker
}

func (c component) Value() string {
	return c.value
}

func (c component) Type() string {
	return c.typ
}

func (c component) Exists() error {
	if c.checker == nil {
		return ErrUnknownComponentExists
	}

	return c.checker.Exists()
}

func WithChecker(c Component, check resourcecheck.ResourceChecker) Component {
	return component{
		value:   c.Value(),
		typ:     c.Type(),
		checker: check,
	}
}

func NewDockerImage(value string) Component {
	return WithChecker(component{
		value: value,
		typ:   "Docker image",
	}, resourcecheck.NewDockerImage(value))
}

func NewDockerImageArchive(value string) Component {
	return WithChecker(component{
		value: value,
		typ:   "Docker image archive",
	}, resourcecheck.NewFile(value))
}

func NewFile(value string) Component {
	return WithChecker(component{
		value: value,
		typ:   "File",
	}, resourcecheck.NewFile(value))
}
