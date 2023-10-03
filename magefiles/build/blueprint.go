package build

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
)

type TargetBlueprint[T fmt.Stringer, E fmt.Stringer, F any] interface {
	Prerequisites() []T
	Artifacts() []E
	Data() F
}

func PrintBlueprint[T fmt.Stringer, E fmt.Stringer, F any](blueprint TargetBlueprint[T, E, F]) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Target info"})
	t.AppendRow(table.Row{"Prerequisites"})
	t.AppendSeparator()
	for _, p := range blueprint.Prerequisites() {
		t.AppendRow(table.Row{p})
	}
	t.AppendSeparator()

	t.AppendRow(table.Row{"Artifacts"})
	t.AppendSeparator()
	for _, a := range blueprint.Artifacts() {
		t.AppendRow(table.Row{a})
	}

	fmt.Println(t.Render())
}
