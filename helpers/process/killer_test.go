package process

import (
	"errors"
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
)

func mockKillerFactory(t *testing.T) (*mockKiller, func()) {
	killerMock := new(mockKiller)

	oldNewKillerFactory := newKillerFactory
	cleanup := func() {
		newKillerFactory = oldNewKillerFactory
		killerMock.AssertExpectations(t)
	}

	newKillerFactory = func(logger Logger, cmd Commander) killer {
		return killerMock
	}

	return killerMock, cleanup
}

func TestDefaultKillWaiter_KillAndWait(t *testing.T) {
	testProcess := &os.Process{Pid: 1234}
	processStoppedErr := errors.New("process stopped properly")

	tests := map[string]struct {
		process          *os.Process
		terminateProcess bool
		forceKillProcess bool
		expectedError    string
	}{
		"process is nil": {
			process:       nil,
			expectedError: "process not started yet",
		},
		"process terminated": {
			process:          testProcess,
			terminateProcess: true,
			expectedError:    processStoppedErr.Error(),
		},
		"process force-killed": {
			process:          testProcess,
			forceKillProcess: true,
			expectedError:    processStoppedErr.Error(),
		},
		"process killing failed": {
			process:       testProcess,
			expectedError: dormantProcessError(testProcess).Error(),
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			waitCh := make(chan error, 1)

			killerMock, cleanup := mockKillerFactory(t)
			defer cleanup()

			loggerMock := new(MockLogger)
			defer loggerMock.AssertExpectations(t)

			commanderMock := new(MockCommander)
			defer commanderMock.AssertExpectations(t)

			commanderMock.On("Process").Return(testCase.process)

			if testCase.process != nil {
				loggerMock.
					On("WithFields", mock.Anything).
					Return(loggerMock)

				terminateCall := killerMock.On("Terminate")
				forceKillCall := killerMock.On("ForceKill").Maybe()

				if testCase.terminateProcess {
					terminateCall.Run(func(_ mock.Arguments) {
						waitCh <- processStoppedErr
					})
				}

				if testCase.forceKillProcess {
					forceKillCall.Run(func(_ mock.Arguments) {
						waitCh <- processStoppedErr
					})
				}
			}

			kw := NewKillWaiter(loggerMock, 10*time.Millisecond, 10*time.Millisecond)
			err := kw.KillAndWait(commanderMock, waitCh)

			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}

			assert.EqualError(t, err, testCase.expectedError)
		})
	}
}

func newKillerWithLoggerAndCommand(t *testing.T, duration string, skipTerminate bool) (killer, *MockLogger, Commander, func()) {
	t.Helper()

	loggerMock := new(MockLogger)
	sleepBinary := prepareTestBinary(t)

	args := []string{duration}
	if skipTerminate {
		args = append(args, "skip-terminate-signals")
	}

	command := NewCmd(sleepBinary, args, CommandOptions{})
	err := command.Start()
	require.NoError(t, err)

	k := newKiller(loggerMock, command)

	cleanup := func() {
		loggerMock.AssertExpectations(t)
		_ = os.RemoveAll(filepath.Dir(sleepBinary))
	}

	return k, loggerMock, command, cleanup
}

func prepareTestBinary(t *testing.T) string {
	t.Helper()

	dir, err := ioutil.TempDir("", strings.Replace(t.Name(), "/", "", -1))
	require.NoError(t, err)
	binaryPath := filepath.Join(dir, strconv.FormatInt(time.Now().UnixNano(), 10))

	// Windows can only have executables ending with `.exe`
	if runtime.GOOS == "windows" {
		binaryPath = fmt.Sprintf("%s.exe", binaryPath)
	}

	_, currentTestFile, _, _ := runtime.Caller(0)
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

				loggerMock.On("Errorln",
					"Failed to terminate process:",
					mock.Anything)
				loggerMock.On("Errorln",
					"Failed to force-kill:",
					mock.Anything)
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
