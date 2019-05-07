package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVaultUserpassAuth_GetPath(t *testing.T) {
	tests := map[string]struct {
		path         string
		expectedPath string
	}{
		"path not defined": {
			expectedPath: DefaultUserpassAuthPath,
		},
		"path defined": {
			path:         "some/path",
			expectedPath: "some/path",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			u := &VaultUserpassAuth{
				Path: test.path,
			}
			assert.Equal(t, test.expectedPath, u.GetPath())
		})
	}
}

func TestVaultTLSAuth_GetPath(t *testing.T) {
	tests := map[string]struct {
		path         string
		expectedPath string
	}{
		"path not defined": {
			expectedPath: DefaultTLSAuthPath,
		},
		"path defined": {
			path:         "some/path",
			expectedPath: "some/path",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			u := &VaultTLSAuth{
				Path: test.path,
			}
			assert.Equal(t, test.expectedPath, u.GetPath())
		})
	}
}

func TestVaultSecretKeys_String(t *testing.T) {
	k := VaultSecretKeys{
		{Key: "key1", EnvName: "env1"},
		{Key: "key2", EnvName: "env2"},
	}

	assert.Equal(t, "[key1=env1 key2=env2]", k.String())
}
