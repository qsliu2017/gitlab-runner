package auth_methods

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

func TestErrFactoryAlreadyRegistered_Error(t *testing.T) {
	assert.Equal(t, `factory for auth method "test-auth" already registered`, NewErrFactoryAlreadyRegistered("test-auth").Error())
}

func TestErrFactoryAlreadyRegistered_Is(t *testing.T) {
	assert.True(t, errors.Is(NewErrFactoryAlreadyRegistered("test-auth"), new(ErrFactoryAlreadyRegistered)))
}

func TestErrFactoryNotRegistered_Error(t *testing.T) {
	assert.Equal(t, `factory for auth method "test-auth" is not registered`, NewErrFactoryNotRegistered("test-auth").Error())
}

func TestErrFactoryNotRegistered_Is(t *testing.T) {
	assert.True(t, errors.Is(NewErrFactoryNotRegistered("test-auth"), new(ErrFactoryNotRegistered)))
}

func TestMustRegisterFactory(t *testing.T) {
	factory := func(path string, data map[string]interface{}) (vault.AuthMethod, error) {
		return new(vault.MockAuthMethod), nil
	}

	tests := map[string]struct {
		register      func()
		panicExpected bool
	}{
		"duplicate factory registration": {
			register: func() {
				MustRegisterFactory("test-auth", factory)
				MustRegisterFactory("test-auth", factory)
			},
			panicExpected: true,
		},
		"successful factory registration": {
			register: func() {
				MustRegisterFactory("test-auth", factory)
				MustRegisterFactory("test-auth-2", factory)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			oldReg := reg
			defer func() {
				reg = oldReg
			}()
			reg = nil

			if tt.panicExpected {
				assert.Panics(t, tt.register)
				return
			}
			assert.NotPanics(t, tt.register)
		})
	}
}

func TestGetFactory(t *testing.T) {
	oldReg := reg
	defer func() {
		reg = oldReg
	}()
	reg = nil

	require.NotPanics(t, func() {
		MustRegisterFactory("test-auth", func(path string, data map[string]interface{}) (vault.AuthMethod, error) {
			return new(vault.MockAuthMethod), nil
		})
	})

	tests := map[string]struct {
		engineName    string
		expectedError error
	}{
		"factory found": {
			engineName:    "not-existing-auth",
			expectedError: new(ErrFactoryNotRegistered),
		},
		"factory not found": {
			engineName: "test-auth",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			factory, err := GetFactory(tt.engineName)
			if tt.expectedError != nil {
				assert.True(t, errors.Is(err, tt.expectedError))
				assert.Nil(t, factory)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, factory)
		})
	}
}
