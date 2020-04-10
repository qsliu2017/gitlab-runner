package secret_engines

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

func TestErrFactoryAlreadyRegistered_Error(t *testing.T) {
	assert.Equal(t, `factory for engine "test-engine" already registered`, NewErrFactoryAlreadyRegistered("test-engine").Error())
}

func TestErrFactoryAlreadyRegistered_Is(t *testing.T) {
	assert.True(t, errors.Is(NewErrFactoryAlreadyRegistered("test-engine"), new(ErrFactoryAlreadyRegistered)))
}

func TestErrFactoryNotRegistered_Error(t *testing.T) {
	assert.Equal(t, `factory for engine "test-engine" is not registered`, NewErrFactoryNotRegistered("test-engine").Error())
}

func TestErrFactoryNotRegistered_Is(t *testing.T) {
	assert.True(t, errors.Is(NewErrFactoryNotRegistered("test-engine"), new(ErrFactoryNotRegistered)))
}

func TestMustRegisterFactory(t *testing.T) {
	factory := func(client vault.Client, path string) vault.SecretEngine {
		return new(vault.MockSecretEngine)
	}

	tests := map[string]struct {
		register      func()
		panicExpected bool
	}{
		"duplicate factory registration": {
			register: func() {
				MustRegisterFactory("test-engine", factory)
				MustRegisterFactory("test-engine", factory)
			},
			panicExpected: true,
		},
		"successful factory registration": {
			register: func() {
				MustRegisterFactory("test-engine", factory)
				MustRegisterFactory("test-engine-2", factory)
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
		MustRegisterFactory("test-engine", func(client vault.Client, path string) vault.SecretEngine {
			return new(vault.MockSecretEngine)
		})
	})

	tests := map[string]struct {
		engineName    string
		expectedError error
	}{
		"factory found": {
			engineName:    "not-existing-engine",
			expectedError: new(ErrFactoryNotRegistered),
		},
		"factory not found": {
			engineName: "test-engine",
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
