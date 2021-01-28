//go:build !integration
// +build !integration

package retry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoff_Run(t *testing.T) {
	sleepTime := 50 * time.Millisecond

	mockRetryable := new(MockRetryable)
	defer mockRetryable.AssertExpectations(t)

	mockRetryable.On("Run").Return(assert.AnError).Times(2)
	mockRetryable.On("ShouldRetry", 1, assert.AnError).Return(true).Once()
	mockRetryable.On("ShouldRetry", 2, assert.AnError).Return(false).Once()

	retry := NewBackoff(mockRetryable)
	retry.backoff.Min = sleepTime
	retry.backoff.Max = sleepTime

	start := time.Now()
	err := retry.Run()
	assert.True(t, time.Since(start) >= sleepTime)
	assert.ErrorIs(t, err, assert.AnError)
	var e *ErrRetriesExceeded
	assert.ErrorAs(t, err, &e)
}
