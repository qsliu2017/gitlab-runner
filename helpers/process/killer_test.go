package process

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func mockKillerFactory(t *testing.T) (*mockKiller, func()) {
	killerMock := new(mockKiller)

	oldNewKillerFactory := newKillerFactory
	cleanup := func() {
		newKillerFactory = oldNewKillerFactory
		killerMock.AssertExpectations(t)
	}

	newKillerFactory = func(logger common.BuildLogger, process *os.Process) killer {
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
			killerMock, cleanup := mockKillerFactory(t)
			defer cleanup()

			logger := common.NewBuildLogger(nil, logrus.NewEntry(logrus.New()))
			kw := NewKillWaiter(logger, 10*time.Millisecond, 10*time.Millisecond)

			waitCh := make(chan error, 1)

			if testCase.process != nil {
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

			err := kw.KillAndWait(testCase.process, waitCh)

			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}

			assert.EqualError(t, err, testCase.expectedError)
		})
	}
}
