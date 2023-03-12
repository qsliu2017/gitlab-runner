//go:build !integration

package helpers

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func TestWriteFile(t *testing.T) {
	const expectedContents = "foobar"

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	tests := []struct {
		path    string
		mode    string
		success bool
	}{
		{
			path:    "",
			success: false,
		},
		{
			path:    "",
			mode:    "777",
			success: false,
		},
		{
			path:    "valid",
			success: true,
		},
		{
			path:    "valid-with-mode",
			mode:    "777",
			success: true,
		},
		{
			path:    "invalid-with-mode",
			mode:    "not-a-mode",
			success: false,
		},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("name:%v mode:%v success:%v", tc.path, tc.mode, tc.success), func(t *testing.T) {
			stdin = strings.NewReader(expectedContents)
			defer func() {
				stdin = os.Stdin
			}()

			tc.path = filepath.Join(t.TempDir(), tc.path)

			fs := flag.NewFlagSet("write-file", flag.PanicOnError)
			require.NoError(t, fs.Parse([]string{tc.path}))

			cliContext := cli.NewContext(nil, fs, nil)

			cmd := newWriteFileCommand()
			cmd.Mode = tc.mode

			if tc.success {
				require.NotPanics(t, func() {
					cmd.Execute(cliContext)
				})

				buf, err := os.ReadFile(tc.path)
				require.NoError(t, err)
				require.Equal(t, expectedContents, string(buf))

				if tc.mode != "" {
					fi, err := os.Stat(tc.path)
					require.NoError(t, err)
					require.Equal(t, tc.mode, fmt.Sprintf("%o", fi.Mode()))
				}
			} else {
				require.Panics(t, func() {
					cmd.Execute(cliContext)
				})
			}
		})
	}
}
