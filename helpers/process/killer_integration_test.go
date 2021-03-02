// +build integration

package process_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func newKillerWithLoggerAndCommand(
	t *testing.T,
	duration string,
	skipTerminate bool,
) (process.Killer, *process.MockLogger, process.Commander, func()) {
	t.Helper()

	loggerMock := new(process.MockLogger)
	sleepBinary := prepareTestBinary(t)

	args := []string{duration}
	if skipTerminate {
		args = append(args, "skip-terminate-signals")
	}

	command := process.NewOSCmd(sleepBinary, args, process.CommandOptions{})
	err := command.Start()
	require.NoError(t, err)

	k := process.NewKillerForTest(loggerMock, command)

	cleanup := func() {
		loggerMock.AssertExpectations(t)
		err = os.RemoveAll(filepath.Dir(sleepBinary))
		if err != nil {
			t.Logf("Failed to cleanup files %q: %v", filepath.Dir(sleepBinary), err)
		}
	}

	return k, loggerMock, command, cleanup
}

func prepareTestBinary(t *testing.T) string {
	t.Helper()

	dir, err := ioutil.TempDir("", strings.ReplaceAll(t.Name(), "/", ""))
	require.NoError(t, err)
	binaryPath := filepath.Join(dir, strconv.FormatInt(time.Now().UnixNano(), 10))

	// Windows can only have executables ending with `.exe`
	if runtime.GOOS == "windows" {
		binaryPath = fmt.Sprintf("%s.exe", binaryPath)
	}

	_, currentTestFile, _, _ := runtime.Caller(0) // nolint:dogsled
	sleepCommandSource := filepath.Clean(filepath.Join(filepath.Dir(currentTestFile), "testdata", "sleep", "main.go"))

	command := exec.Command("go", "build", "-o", binaryPath, sleepCommandSource)
	err = command.Run()
	require.NoError(t, err)

	return binaryPath
}

// Unix and Windows have different test cases expecting different data, check
// killer_unix_test.go and killer_windows_test.go for each system test case.
type testKillerTestCase struct {
	alreadyStopped bool
	skipTerminate  bool
	expectedError  string
}

func TestKiller(t *testing.T) {
	sleepDuration := "3s"

	for testName, testCase := range testKillerTestCases() {
		t.Run(testName, func(t *testing.T) {
			k, loggerMock, cmd, cleanup := newKillerWithLoggerAndCommand(t, sleepDuration, testCase.skipTerminate)
			defer cleanup()

			waitCh := make(chan error)

			if testCase.alreadyStopped {
				_ = cmd.Process().Kill()

				loggerMock.On(
					"Warn",
					"Failed to terminate process:",
					mock.Anything,
				)
				loggerMock.On(
					"Warn",
					"Failed to force-kill:",
					mock.Anything,
				)
			}

			go func() {
				waitCh <- cmd.Wait()
			}()

			time.Sleep(1 * time.Second)
			k.Terminate()

			err := <-waitCh
			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}

			assert.EqualError(t, err, testCase.expectedError)
		})
	}
}

func TestCommandContextCancel(t *testing.T) {
	sleepBinary := prepareTestBinary(t)
	defer func() {
		err := os.RemoveAll(filepath.Dir(sleepBinary))
		if err != nil {
			t.Logf("Failed to cleanup files %q: %v", filepath.Dir(sleepBinary), err)
		}
	}()

	tests := map[string]struct {
		args          []string
		delayCancel   time.Duration
		expectedError map[string]string
	}{
		"exits on its own": {
			args: []string{"1ms"},
		},
		"exits gracefully after being signaled": {
			args:        []string{"1s"},
			delayCancel: 100 * time.Millisecond,
			expectedError: map[string]string{
				"windows": "",
				"unix":    "exit status 1",
			},
		},
		"ignores graceful termination, exits on its own": {
			args:        []string{"1s", "skip-terminate-signals"},
			delayCancel: 100 * time.Millisecond,
		},
		"ignores graceful termination, is forced to exit": {
			args:        []string{"2s", "skip-terminate-signals"},
			delayCancel: 100 * time.Millisecond,
			expectedError: map[string]string{
				"windows": "",
				"unix":    "signal: killed",
			},
		},
		"exits on its own, but then canceled": {
			args:        []string{"1s", "skip-terminate-signals"},
			delayCancel: 2 * time.Second,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			logger := new(MockLogger)
			logger.
				On("WithFields", mock.Anything).
				Return(logger).
				Maybe()

			defer logger.AssertExpectations(t)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			command := NewOSCmd(ctx, sleepBinary, tc.args, CommandOptions{
				Logger:              logger,
				GracefulKillTimeout: 1500 * time.Millisecond,
				ForceKillTimeout:    time.Millisecond,
				Stderr:              os.Stderr,
				Stdout:              os.Stdout,
			})

			err := command.Start()
			require.NoError(t, err)

			waitCh := make(chan error)
			go func() {
				waitCh <- command.Wait()
			}()

			done := make(chan struct{})
			if tc.delayCancel > 0 {
				time.AfterFunc(tc.delayCancel, func() {
					cancel()
					done <- struct{}{}
				})
				defer func() { <-done }()
			}

			err = <-waitCh
			if len(tc.expectedError) == 0 {
				assert.NoError(t, err)
				return
			}

			os := "unix"
			if runtime.GOOS == "windows" {
				os = "windows"
			}
			assert.EqualError(t, err, tc.expectedError[os])
		})
	}
}
