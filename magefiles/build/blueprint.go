package build

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/samber/lo"
	"sort"
)

type BlueprintDependency interface {
	String() string
}

type BlueprintArtifact interface {
	String() string
}

type StringDependency string

func (b StringDependency) String() string {
	return string(b)
}

type StringArtifact string

func (b StringArtifact) String() string {
	return string(b)
}

type TargetBlueprint[T BlueprintDependency, E BlueprintArtifact, F any] interface {
	Dependencies() []T
	Artifacts() []E
	Data() F
}

func PrintBlueprint[T BlueprintDependency, E BlueprintArtifact, F any](blueprint TargetBlueprint[T, E, F]) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Target info"})
	t.AppendRow(table.Row{"Dependencies"})
	t.AppendSeparator()
	deps := lo.Uniq(lo.Map(blueprint.Dependencies(), func(item T, _ int) string {
		return item.String()
	}))
	sort.Strings(deps)
	for _, p := range deps {
		t.AppendRow(table.Row{p})
	}
	t.AppendSeparator()

	t.AppendRow(table.Row{"Artifacts"})
	t.AppendSeparator()
	artifacts := lo.Uniq(lo.Map(blueprint.Artifacts(), func(item E, _ int) string {
		return item.String()
	}))
	for _, a := range artifacts {
		t.AppendRow(table.Row{a})
	}

	fmt.Println(t.Render())
}
