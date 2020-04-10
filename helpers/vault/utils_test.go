package vault

import (
	"errors"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
)

func TestTrimSlashes(t *testing.T) {
	tests := map[string]string{
		"test":      "test",
		"test/":     "test",
		"test/////": "test",
		"/test":     "test",
		"////test":  "test",
		"test/test": "test/test",
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt, TrimSlashes(tn))
		})
	}
}

func TestUnwrapAPIResponseError(t *testing.T) {
	tests := map[string]struct {
		err           error
		expectedError error
	}{
		"nil error": {
			err:           nil,
			expectedError: nil,
		},
		"non-API error": {
			err:           assert.AnError,
			expectedError: assert.AnError,
		},
		"API error": {
			err:           &api.ResponseError{StatusCode: -1, Errors: []string{"test1", "test2"}},
			expectedError: new(errUnwrappedAPIResponse),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			err := unwrapAPIResponseError(tt.err)
			assert.True(t, errors.Is(err, tt.expectedError))
		})
	}
}

func TestErrUnwrappedAPIResponse_Error(t *testing.T) {
	err := newErrUnwrappedAPIResponse(-1, []string{"test1", "test2"})
	assert.Equal(t, "api error: status code -1: test1, test2", err.Error())
}

func TestErrUnwrappedAPIResponse_Is(t *testing.T) {
	assert.True(t, errors.Is(newErrUnwrappedAPIResponse(-1, []string{"test1", "test2"}), new(errUnwrappedAPIResponse)))
}
