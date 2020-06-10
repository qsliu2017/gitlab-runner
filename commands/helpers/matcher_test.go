package helpers

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func matchFilesystemSetup(t *testing.T, fn func(dir string)) {
	files := []string{
		"/bar.txt",
		"/build/foo.txt",
		"/build/a/foo.txt",
		"/build/a/b/foo.txt",
		"/build/a/b/c/foo.txt",
	}

	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	for _, file := range files {
		pathname := filepath.Join(dir, file)

		require.NoError(t, os.MkdirAll(filepath.Dir(pathname), 0700))
		require.NoError(t, ioutil.WriteFile(pathname, nil, 0600))
	}

	fn(dir)
}

func TestMatchRootPatterns(t *testing.T) {
	patterns := []string{".", "./", "*", "**", "**/*"}

	matchFilesystemSetup(t, func(dir string) {
		dir = filepath.Join(dir, "build")

		expected := []string{
			"foo.txt",
			"a",
			"a/foo.txt",
			"a/b",
			"a/b/foo.txt",
			"a/b/c",
			"a/b/c/foo.txt",
		}

		for _, pattern := range patterns {
			t.Run(pattern, func(t *testing.T) {
				files, stats, err := match(context.Background(), dir, []string{pattern}, nil, false)
				assert.NoError(t, err)
				assert.Len(t, expected, stats.Files)

				for _, name := range expected {
					assert.Contains(t, files, filepath.Join(dir, name))
				}
			})
		}
	})
}

func TestMatchRecursive(t *testing.T) {
	matchFilesystemSetup(t, func(dir string) {
		dir = filepath.Join(dir, "build")

		includes := []string{
			"../bar.txt",
			"a/b/",
			"a/b/c",
		}

		expected := []string{
			"a/b",
			"a/b/foo.txt",
			"a/b/c",
			"a/b/c/foo.txt",
		}

		files, stats, err := match(context.Background(), dir, includes, nil, false)
		assert.NoError(t, err)
		assert.Len(t, expected, stats.Files)

		for _, name := range expected {
			assert.Contains(t, files, filepath.Join(dir, name))
		}
	})
}

func TestMatchRecursiveExclude(t *testing.T) {
	matchFilesystemSetup(t, func(dir string) {
		dir = filepath.Join(dir, "build")

		includes := []string{
			"a/foo.txt", // includes a/foo.txt
			"a/b/*",     // includes a/b/*
		}

		excludes := []string{
			"a/b/*/", // excludes *directories* but not files within a/b
		}

		expected := []string{
			"a/foo.txt",
			"a/b",
			"a/b/foo.txt",
		}

		files, stats, err := match(context.Background(), dir, includes, excludes, false)
		assert.NoError(t, err)
		assert.Len(t, expected, stats.Files)

		for _, name := range expected {
			assert.Contains(t, files, filepath.Join(dir, name))
		}
	})
}

func testMatchDeprecate14Unix(t *testing.T) {
	dir := "/system/runner/build"
	patterns := []string{
		`*/**`,
		`pattern`,
		`/**`,
		`/outside/wild/**`,
		`/outside/plain`,
		`/system`,
		`/system/runner`,
		`/system/runner/build/inside/wild/**`,
		`/system/runner/build/inside/plain`,
		`/system/runner/build/inside/\escaped`,
	}
	expected := []string{
		`*/**`,
		`pattern`,
		`./`,
		`./`,
		`inside/wild/**`,
		`inside/plain`,
		`inside/\escaped`,
	}

	assert.Equal(t, expected, deprecated14PatternCheck(dir, patterns))
}

func testMatchDeprecate14Windows(t *testing.T) {
	dir := "C:\\system\\runner\\build"
	patterns := []string{
		`*\**`,
		`pattern`,
		`C:\**`,
		`C:\outside\wild\**`,
		`C:\outside\plain`,
		`C:\system`,
		`C:\system\runner`,
		`C:\system\runner\build\inside\wild\**`,
		`C:\system\runner\build\inside\plain`,
		`C:\system\runner\build\inside\\escaped`,
		`Z:\system\runner`,
	}
	expected := []string{
		`*/**`,
		`pattern`,
		`./`,
		`./`,
		`inside/wild/**`,
		`inside/plain`,
		`inside/escaped`,
	}

	assert.Equal(t, expected, deprecated14PatternCheck(dir, patterns))
}

func TestMatchDeprecate14Patterns(t *testing.T) {
	if runtime.GOOS == "windows" {
		testMatchDeprecate14Windows(t)
	} else {
		testMatchDeprecate14Unix(t)
	}
}
