package mageutils

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/samber/lo"
	"os"
	"sort"
	"sync"
)

var used = map[string]bool{}
var usedLock sync.Mutex

func PrintUsedVariables(verbose bool) {
	if !verbose {
		return
	}

	usedLock.Lock()
	defer usedLock.Unlock()

	keys := lo.Keys(used)
	sort.Strings(keys)

	t := table.NewWriter()
	t.AppendHeader(table.Row{"Used environment variables"})
	for _, k := range keys {
		t.AppendRow(table.Row{k})
	}

	fmt.Println(t.Render())
}

func Env(env string) string {
	usedLock.Lock()
	used[env] = true
	usedLock.Unlock()

	return os.Getenv(env)
}

func EnvOrDefault(env, def string) string {
	return EnvFallbackOrDefault(env, "", def)
}

func EnvFallbackOrDefault(env, fallback, def string) string {
	val := Env(env)
	if val != "" {
		return val
	}
	if fallback != "" {
		val = Env(fallback)
		if val != "" {
			return val
		}
	}

	return def
}

type Once[T any] struct {
	val T

	o sync.Once
}

func (o *Once[T]) Do(fn func() (T, error)) T {
	o.o.Do(func() {
		var err error
		o.val, err = fn()

		if err != nil {
			panic(err)
		}
	})

	return o.val
}
