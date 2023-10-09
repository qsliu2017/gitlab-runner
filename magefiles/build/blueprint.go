package build

import (
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

type TargetBlueprint[T Component, E Component, F any] interface {
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

func PrintBlueprint[T Component, E Component, F any](blueprint TargetBlueprint[T, E, F]) TargetBlueprint[T, E, F] {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Target info"})

	t.AppendRow(table.Row{"Dependencies", "Type"})
	t.AppendSeparator()
	t.AppendRows(rowsFromComponents(blueprint.Dependencies()))

	t.AppendSeparator()

	t.AppendRow(table.Row{"Artifacts", "Type"})
	t.AppendSeparator()
	t.AppendRows(rowsFromComponents(blueprint.Artifacts()))
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

func rowsFromComponents[T Component](components []T) []table.Row {
	deps := lo.Reduce(components, func(acc map[string]string, item T, _ int) map[string]string {
		acc[item.Value()] = item.Type()
		return acc
	}, map[string]string{})
	values := lo.Keys(deps)
	sort.Strings(values)

	return lo.Map(values, func(value string, _ int) table.Row {
		depType := deps[value]
		return table.Row{value, depType}
	})
}
