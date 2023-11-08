package build

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

// Read magefiles/docs/writing_mage_targets.md for details of blueprints

const (
	// Don't look at me, the linter made me do it
	messageYes = "Yes"
	messageNo  = "No"
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

func (e BlueprintEnv) Var(key string) env.Variable {
	return e.env[key]
}

func (e BlueprintEnv) ValueFrom(env string) string {
	v, ok := e.env[env]
	if !ok {
		fmt.Printf("WARN: Accessing a variable that's not defined in the blueprint: %q\n", env)
		return ""
	}

	return mageutils.EnvFallbackOrDefault(v.Key, v.Fallback, v.Default)
}

func (e BlueprintEnv) Value(env env.Variable) string {
	return e.ValueFrom(env.Key)
}

func (e BlueprintEnv) Int(env env.Variable) int {
	value, _ := strconv.Atoi(e.Value(env))
	return value
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

	t.AppendRow(table.Row{"Environment variable", "Is set", "Is default"})
	t.AppendSeparator()
	t.AppendRows(rowsFromEnv(blueprint.Env()))

	fmt.Println(t.Render())

	return blueprint
}

func CheckComponents[T Component](components []T) map[string]lo.Tuple2[string, error] {
	deps := make(map[string]lo.Tuple2[string, error])
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, c := range components {
		wg.Add(1)
		go func(c Component) {
			// The exists check could be slow so let's do it concurrently
			// with a bit of good old school Go code
			exists := NewResourceChecker(c).Exists()
			mu.Lock()
			deps[c.Value()] = lo.Tuple2[string, error]{
				A: c.Type(),
				B: exists,
			}
			mu.Unlock()
			wg.Done()
		}(c)
	}

	wg.Wait()

	return deps
}

func RowsFromCheckedComponents(deps map[string]lo.Tuple2[string, error]) []table.Row {
	values := lo.Keys(deps)
	sort.Strings(values)

	return lo.Map(values, func(value string, _ int) table.Row {
		dep := deps[value]

		existsMessage := messageYes
		if dep.B != nil {
			existsMessage = color.New(color.FgRed).Sprint(dep.B.Error())
		}

		return table.Row{value, dep.A, existsMessage}
	})
}

func rowsFromComponents[T Component](components []T) []table.Row {
	return RowsFromCheckedComponents(CheckComponents(components))
}

func rowsFromEnv(blueprintEnv BlueprintEnv) []table.Row {
	envs := lo.Keys(blueprintEnv.env)
	sort.Strings(envs)

	return lo.Map(envs, func(key string, _ int) table.Row {
		isSet := messageYes
		if blueprintEnv.ValueFrom(key) == "" {
			isSet = color.New(color.FgRed).Sprint(messageNo)
		}

		isDefault := messageYes
		if blueprintEnv.ValueFrom(key) != blueprintEnv.Var(key).Default {
			isDefault = color.New(color.FgYellow).Sprint(messageNo)
		}

		return table.Row{key, isSet, isDefault}
	})
}
