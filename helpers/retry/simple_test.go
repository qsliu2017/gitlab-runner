//go:build !integration
// +build !integration

package retry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRetry_Run(t *testing.T) {
	tests := map[string]struct {
		calls       []error
		shouldRetry bool

		expectedErr error
	}{
		"no error should succeed": {
			calls:       []error{nil},
			shouldRetry: false,

			expectedErr: nil,
		},
		"one error succeed on second call": {
			calls:       []error{assert.AnError, nil},
			shouldRetry: true,

			expectedErr: nil,
		},
		"on error should not retry": {
			calls:       []error{assert.AnError},
			shouldRetry: false,

			expectedErr: assert.AnError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockRetryable := new(MockRetryable)
			defer mockRetryable.AssertExpectations(t)

			for _, e := range tt.calls {
				mockRetryable.On("Run").Return(e).Once()
			}
			mockRetryable.On("ShouldRetry", mock.Anything, mock.Anything).Return(tt.shouldRetry).Maybe()

			retry := NewSimple(mockRetryable)
			err := retry.Run()
			assert.ErrorIs(t, err, tt.expectedErr)
			if tt.expectedErr != nil {
				var e *ErrRetriesExceeded
				if assert.ErrorAs(t, err, &e) {
					assert.Equal(t, len(tt.calls), e.tries)
				}
			}
		})
	}
}
