//go:build !integration

package common

import (
	"regexp"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalConfigParse(t *testing.T) {
	tests := map[string]struct {
		config         string
		validateConfig func(t *testing.T, config *LocalConfig)
		expectedErr    string
	}{
		"parse system_id": {
			config: `
			system_id = "some_system_id"
			`,
			validateConfig: func(t *testing.T, config *LocalConfig) {
				assert.Equal(t, "some_system_id", config.SystemID)
			},
		},
		"parse empty system_id": {
			config: "",
			validateConfig: func(t *testing.T, config *LocalConfig) {
				assert.Empty(t, config.SystemID)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cfg := NewLocalConfig()
			_, err := toml.Decode(tt.config, cfg)
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			if tt.validateConfig != nil {
				tt.validateConfig(t, cfg)
			}
		})
	}
}

func TestEnsureSystemID(t *testing.T) {
	tests := map[string]struct {
		config   string
		assertFn func(t *testing.T, config *LocalConfig)
	}{
		"preserves system_id": {
			config: `
			system_id = "some_system_id"
			`,
			assertFn: func(t *testing.T, config *LocalConfig) {
				assert.Equal(t, "some_system_id", config.SystemID)
			},
		},
		"generates missing system_id": {
			config: "",
			assertFn: func(t *testing.T, config *LocalConfig) {
				assert.Regexp(t, regexp.MustCompile("[rs]_[0-9a-zA-Z]{12}"), config.SystemID)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cfg := NewLocalConfig()
			_, err := toml.Decode(tt.config, cfg)
			require.NoError(t, err)

			err = cfg.EnsureSystemID()
			assert.NoError(t, err)
			if tt.assertFn != nil {
				tt.assertFn(t, cfg)
			}
		})
	}
}
