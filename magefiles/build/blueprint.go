package build

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

type TargetBlueprint[T Component, E Component, F any] interface {
	Dependencies() []T
	Artifacts() []E
	Data() F
	Env() BlueprintEnv
}

type BlueprintEnv struct {
	env map[string]env.Variable
}

func (e BlueprintEnv) All() env.Variables {
	return lo.Values(e.env)
}

func (e BlueprintEnv) ValueFrom(env string) string {
	v, ok := e.env[env]
	if !ok {
		fmt.Printf("WARN: Accessing a variable that's not defined in the blueprint: %q\n", env)
	}

	return mageutils.EnvFallbackOrDefault(v.Key, v.Fallback, v.Default)
}

func (e BlueprintEnv) Value(env env.Variable) string {
	return e.ValueFrom(env.Key)
}

type BlueprintBase struct {
	env BlueprintEnv
}

func NewBlueprintBase(envs ...env.VariableBundle) BlueprintBase {
	e := BlueprintEnv{env: map[string]env.Variable{}}
	for _, v := range envs {
		for _, vv := range v.Variables() {
			e.env[vv.Key] = vv
		}
	}

	return BlueprintBase{
		env: e,
	}
}

func (b BlueprintBase) Env() BlueprintEnv {
	return b.env
}

func PrintBlueprint[T Component, E Component, F any](blueprint TargetBlueprint[T, E, F]) TargetBlueprint[T, E, F] {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Target info"})

	t.AppendRow(table.Row{"Dependency", "Type", "Exists"})
	t.AppendSeparator()
	t.AppendRows(rowsFromComponents(blueprint.Dependencies()))

	t.AppendSeparator()

	t.AppendRow(table.Row{"Artifact", "Type", "Exists"})
	t.AppendSeparator()
	t.AppendRows(rowsFromComponents(blueprint.Artifacts()))
	t.AppendSeparator()

	t.AppendRow(table.Row{"Environment variable", "", "Is Set"})
	t.AppendSeparator()
	t.AppendRows(rowsFromEnv(blueprint.Env()))

	fmt.Println(t.Render())

	return blueprint
}

type dep struct {
	typ    string
	exists error
}

func rowsFromComponents[T Component](components []T) []table.Row {
	deps := lo.Reduce(components, func(acc map[string]dep, item T, _ int) map[string]dep {
		acc[item.Value()] = dep{
			typ:    item.Type(),
			exists: item.Exists(),
		}
		return acc
	}, map[string]dep{})

	values := lo.Keys(deps)
	sort.Strings(values)

	return lo.Map(values, func(value string, _ int) table.Row {
		dep := deps[value]

		existsMessage := "Yes"
		if dep.exists != nil {
			existsMessage = color.New(color.FgRed).Sprint(dep.exists.Error())
		}

		return table.Row{value, dep.typ, existsMessage}
	})
}

func rowsFromEnv(blueprintEnv BlueprintEnv) []table.Row {
	envs := lo.Keys(blueprintEnv.env)
	sort.Strings(envs)
	return lo.Map(envs, func(key string, _ int) table.Row {
		isSet := "Yes"
		val := blueprintEnv.ValueFrom(key)
		if val == "" {
			isSet = color.New(color.FgRed).Sprint("No")
		}

		return table.Row{key, "", isSet}
	})

}
