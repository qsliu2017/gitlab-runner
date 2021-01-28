//go:build !integration
// +build !integration

package retry

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestRetryableDecorator(t *testing.T) {
	var runCalled, shouldRetryCalled bool

	r := NewRetryableDecorator(func() error {
		runCalled = true
		return assert.AnError
	}, func(tries int, err error) bool {
		shouldRetryCalled = true
		return true
	})

	assert.False(t, runCalled)
	assert.Equal(t, assert.AnError, r.Run())
	assert.True(t, runCalled)
	assert.False(t, shouldRetryCalled)
	assert.True(t, r.ShouldRetry(0, nil))
	assert.True(t, shouldRetryCalled)
}

func TestRetryableLogrusDecorator(t *testing.T) {
	mr := new(MockRetryable)
	defer mr.AssertExpectations(t)

	mr.On("Run").Return(assert.AnError).Once()
	mr.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()

	logger, hook := test.NewNullLogger()
	r := WithLogrus(mr, logger.WithContext(context.Background()))

	assert.Equal(t, assert.AnError, r.Run())
	assert.True(t, r.ShouldRetry(0, nil))
	assert.Len(t, hook.Entries, 1)
}

func TestRetryableBuildLoggerDecorator(t *testing.T) {
	mr := new(MockRetryable)
	defer mr.AssertExpectations(t)

	mr.On("Run").Return(assert.AnError).Once()
	mr.On("ShouldRetry", mock.Anything, mock.Anything).Return(true).Once()

	logger, hook := test.NewNullLogger()
	buildLogger := common.NewBuildLogger(nil, logger.WithContext(context.Background()))
	r := WithBuildLog(mr, &buildLogger)

	assert.Equal(t, assert.AnError, r.Run())
	assert.True(t, r.ShouldRetry(0, nil))
	assert.Len(t, hook.Entries, 1)
}
