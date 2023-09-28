//go:build mage

package mageutils

import (
	"os"
	"sync"
)

func EnvOrDefault(env, def string) string {
	return EnvFallbackOrDefault(env, "", def)
}

func EnvFallbackOrDefault(env, fallback, def string) string {
	val := os.Getenv(env)
	if val != "" {
		return val
	}
	if fallback != "" {
		val = os.Getenv(fallback)
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
