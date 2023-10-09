package build

import "github.com/samber/lo"

type Component interface {
	Value() string
	Type() string
}

type component lo.Tuple2[string, string]

func (c component) Value() string {
	return c.A
}

func (c component) Type() string {
	return c.B
}

func NewDockerImage(value string) Component {
	return component{
		A: value,
		B: "Docker Image",
	}
}

func NewDockerImageArchive(value string) Component {
	return component{
		A: value,
		B: "Docker Image Archive",
	}
}

func NewFile(value string) Component {
	return component{
		A: value,
		B: "File",
	}
}
