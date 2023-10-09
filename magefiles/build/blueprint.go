package build

import (
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
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
	Env() BlueprintEnv
}

type BlueprintEnv map[string]env.Variable

func (e BlueprintEnv) Value(env string) string {
	v, ok := e[env]
	if !ok {
		fmt.Printf("WARN: Accessing a variable that's not defined in the blueprint: %q\n", env)
	}

	return v.Value
}

type BlueprintBase struct {
	env BlueprintEnv
}

func NewBlueprintBase(envs ...env.VariableBundle) BlueprintBase {
	env := BlueprintEnv{}
	for _, v := range envs {
		for _, vv := range v.Variables() {
			env[vv.Key] = vv
		}
	}

	return BlueprintBase{
		env: env,
	}
}

func (b BlueprintBase) Env() BlueprintEnv {
	return b.env
}

func PrintBlueprint[T BlueprintDependency, E BlueprintArtifact, F any](blueprint TargetBlueprint[T, E, F]) TargetBlueprint[T, E, F] {
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
	t.AppendSeparator()

	t.AppendRow(table.Row{"Environment variables"})
	t.AppendSeparator()
	envs := lo.Keys(blueprint.Env())
	sort.Strings(envs)
	for _, e := range envs {
		t.AppendRow(table.Row{e})
	}

	fmt.Println(t.Render())

	return blueprint
}
