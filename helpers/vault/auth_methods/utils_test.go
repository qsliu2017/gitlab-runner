package auth_methods

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrMissingRequiredConfigurationKey_Error(t *testing.T) {
	assert.Equal(t, `missing required auth method configuration key "test-key"`, NewErrMissingRequiredConfigurationKey("test-key").Error())
}

func TestErrMissingRequiredConfigurationKey_Is(t *testing.T) {
	assert.True(t, errors.Is(NewErrMissingRequiredConfigurationKey("test-key"), new(ErrMissingRequiredConfigurationKey)))
}

func TestFilterAuthenticationData(t *testing.T) {
	requiredKeys := []string{"required1", "required2"}
	allowedKeys := []string{"required1", "required2", "allowed1", "allowed2"}

	tests := map[string]struct {
		data          map[string]interface{}
		expectedData  map[string]interface{}
		expectedError error
	}{
		"missing required field": {
			data: map[string]interface{}{
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
			expectedError: new(ErrMissingRequiredConfigurationKey),
		},
		"missing allowed field": {
			data: map[string]interface{}{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
			},
			expectedData: map[string]interface{}{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
			},
		},
		"unexpected field used": {
			data: map[string]interface{}{
				"required1":   "test",
				"required2":   "test",
				"allowed1":    "test",
				"allowed2":    "test",
				"unexpected1": "test",
				"unexpected2": "test",
			},
			expectedData: map[string]interface{}{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
		},
		"only required and allowed fields": {
			data: map[string]interface{}{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
			expectedData: map[string]interface{}{
				"required1": "test",
				"required2": "test",
				"allowed1":  "test",
				"allowed2":  "test",
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			data, err := FilterAuthenticationData(requiredKeys, allowedKeys, tt.data)

			if tt.expectedError != nil {
				assert.True(t, errors.Is(err, tt.expectedError))
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedData, data)
		})
	}
}
