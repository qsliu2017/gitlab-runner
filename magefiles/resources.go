package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/magefile/mage/mg"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/iter"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
)

type Resources mg.Namespace

func (Resources) Verify(typ string) error {
	rows, err := verify(build.ReleaseArtifactsPath(typ))
	renderTable(rows)
	return err
}

func verify(f string) ([]table.Row, error) {
	if _, err := os.Stat(f); os.IsNotExist(err) {
		return nil, err
	}

	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	var m []map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}

	// TODO: This could probably be better but I am tired now
	c := lo.Map(m, func(m map[string]string, _ int) build.Component {
		return build.NewComponent(m["Value"], m["Type"])
	})

	checked := build.CheckComponents(c)
	rows := build.RowsFromCheckedComponents(checked)
	errs := lo.FilterMap(lo.Values(checked), func(t lo.Tuple2[string, error], _ int) (error, bool) {
		return t.B, t.B != nil
	})
	if len(errs) == 0 {
		return rows, nil
	}

	return rows, errors.New("there were errors in the checked resources")
}

func renderTable(rows []table.Row) {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"Resources status"})
	t.AppendSeparator()

	t.AppendRow(table.Row{"Resource", "Type", "Exists"})
	t.AppendSeparator()

	t.AppendRows(rows)

	fmt.Println(t.Render())
}

func (Resources) VerifyAll() error {
	// TODO: verify that the resources exported match the expected blueprint

	dir := filepath.Dir(build.ReleaseArtifactsPath(""))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	mapper := iter.Mapper[os.DirEntry, []table.Row]{
		MaxGoroutines: config.Concurrency,
	}

	rows, err := mapper.MapErr(entries, func(entry *os.DirEntry) ([]table.Row, error) {
		if (*entry).IsDir() {
			return nil, nil
		}

		f := (*entry).Name()
		return verify(filepath.Join(dir, f))
	})

	renderTable(lo.Flatten(rows))

	return err
}
