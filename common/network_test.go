package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheCheckPolicy(t *testing.T) {
	for num, tc := range []struct {
		object      CachePolicy
		subject     CachePolicy
		expected    bool
		expectErr   bool
		description string
	}{
		{CachePolicyPullPush, CachePolicyPull, true, false, "pull-push allows pull"},
		{CachePolicyPullPush, CachePolicyPush, true, false, "pull-push allows push"},
		{CachePolicyUndefined, CachePolicyPull, true, false, "undefined allows pull"},
		{CachePolicyUndefined, CachePolicyPush, true, false, "undefined allows push"},
		{CachePolicyPull, CachePolicyPull, true, false, "pull allows pull"},
		{CachePolicyPull, CachePolicyPush, false, false, "pull forbids push"},
		{CachePolicyPush, CachePolicyPull, false, false, "push forbids pull"},
		{CachePolicyPush, CachePolicyPush, true, false, "push allows push"},
		{"unknown", CachePolicyPull, false, true, "unknown raises error on pull"},
		{"unknown", CachePolicyPush, false, true, "unknown raises error on push"},
	} {
		cache := Cache{Policy: tc.object}

		result, err := cache.CheckPolicy(tc.subject)
		if tc.expectErr {
			assert.Errorf(t, err, "case %d: %s", num, tc.description)
		} else {
			assert.NoErrorf(t, err, "case %d: %s", num, tc.description)
		}

		assert.Equal(t, tc.expected, result, "case %d: %s", num, tc.description)
	}
}

func TestJobResponse_GetVault(t *testing.T) {
	vaultSpec := Vault{
		"server_1": VaultServerDef{
			Server: VaultServer{
				URL: "https://vault-1:8200",
				Auth: VaultAuth{
					Name: "jwt",
					Path: "jwt",
					Data: VaultAuthData{
						"jwt": "$CI_JOB_JWT",
					},
				},
			},
			Secrets: VaultSecrets{
				"SOME_SECRET_1": {
					Engine: VaultEngine{
						Name: "kv2",
						Path: "secrets",
					},
					Path:     "path",
					Strategy: "read|delete",
				},
			},
		},
		"server_2": VaultServerDef{
			Server: VaultServer{
				URL: "https://vault-2:8200",
				Auth: VaultAuth{
					Name: "jwt",
					Path: "some/jwt",
					Data: VaultAuthData{
						"jwt":  "example-token",
						"role": "custom_role",
					},
				},
			},
			Secrets: VaultSecrets{
				"SOME_SECRET_2": {
					Engine: VaultEngine{
						Name: "kv1",
						Path: "kv1",
					},
					Path:     "path",
					Strategy: "read",
				},
			},
		},
	}
	vaultSpecYAML := `
server_1:
  server:
    url: https://vault-1:8200
    auth:
      name: jwt
      path: jwt
      data:
        jwt: $CI_JOB_JWT
  secrets:
    SOME_SECRET_1:
      engine:
        name: kv2
        path: secrets
      path: path
      strategy: read|delete
server_2:
  server:
    url: https://vault-2:8200
    auth:
      name: jwt
      path: some/jwt
      data:
        jwt: example-token
        role: custom_role
  secrets:
    SOME_SECRET_2:
      engine:
        name: kv1
        path: kv1
      path: path
      strategy: read
`
	vaultSpecJSON := `
{
  "server_1": {
    "server": {
      "url": "https://vault-1:8200",
      "auth": {
        "name": "jwt",
        "path": "jwt",
        "data": {
          "jwt": "$CI_JOB_JWT"
        }
      }
    },
    "secrets": {
      "SOME_SECRET_1": {
        "engine": {
	      "name": "kv2",
          "path": "secrets"
        },
        "path": "path",
        "strategy": "read|delete"
      }
    }
  },
  "server_2": {
    "server": {
      "url": "https://vault-2:8200",
      "auth": {
        "name": "jwt",
        "path": "some/jwt",
        "data": {
          "jwt": "example-token",
          "role": "custom_role"
        }
      }
    },
    "secrets": {
      "SOME_SECRET_2": {
        "engine": {
	      "name": "kv1",
          "path": "kv1"
        },
        "path": "path",
        "strategy": "read"
      }
    }
  }
}
`

	tests := map[string]struct {
		job               *JobResponse
		expectedVaultSpec Vault
	}{
		"no vault specification": {
			job:               &JobResponse{},
			expectedVaultSpec: nil,
		},
		"vault specification received explicitly": {
			job: &JobResponse{
				Vault: &vaultSpec,
			},
			expectedVaultSpec: vaultSpec,
		},
		"vault specification received with empty variable": {
			job: &JobResponse{
				Variables: JobVariables{
					{Key: VaultDefinitionVariable, Value: ""},
				},
			},
			expectedVaultSpec: nil,
		},
		"vault specification received with variable as YAML": {
			job: &JobResponse{
				Variables: JobVariables{
					{Key: VaultDefinitionVariable, Value: vaultSpecYAML},
				},
			},
			expectedVaultSpec: vaultSpec,
		},
		"vault specification received with variable as JSON": {
			job: &JobResponse{
				Variables: JobVariables{
					{Key: VaultDefinitionVariable, Value: vaultSpecJSON},
				},
			},
			expectedVaultSpec: vaultSpec,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			vault := tt.job.GetVault()
			if tt.expectedVaultSpec == nil {
				assert.Nil(t, vault)
				return
			}
			assert.Equal(t, tt.expectedVaultSpec, *vault)
		})
	}
}

func TestVaultSecret_StrategyHandling(t *testing.T) {
	tests := map[string]struct {
		strategy     string
		assertSecret func(t *testing.T, secret VaultSecret)
	}{
		"strategy not defined": {
			strategy: "",
			assertSecret: func(t *testing.T, secret VaultSecret) {
				assert.True(t, secret.ShouldRead())
				assert.False(t, secret.ShouldWrite())
				assert.False(t, secret.ShouldDelete())
			},
		},
		"read strategy defined explicitly": {
			strategy: "read",
			assertSecret: func(t *testing.T, secret VaultSecret) {
				assert.True(t, secret.ShouldRead())
				assert.False(t, secret.ShouldWrite())
				assert.False(t, secret.ShouldDelete())
			},
		},
		"write strategy defined explicitly": {
			strategy: "write",
			assertSecret: func(t *testing.T, secret VaultSecret) {
				assert.False(t, secret.ShouldRead())
				assert.True(t, secret.ShouldWrite())
				assert.False(t, secret.ShouldDelete())
			},
		},
		"delete strategy defined explicitly": {
			strategy: "delete",
			assertSecret: func(t *testing.T, secret VaultSecret) {
				assert.False(t, secret.ShouldRead())
				assert.False(t, secret.ShouldWrite())
				assert.True(t, secret.ShouldDelete())
			},
		},
		"multiple strategies defined": {
			strategy: "read|delete",
			assertSecret: func(t *testing.T, secret VaultSecret) {
				assert.True(t, secret.ShouldRead())
				assert.False(t, secret.ShouldWrite())
				assert.True(t, secret.ShouldDelete())
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			tt.assertSecret(t, VaultSecret{Strategy: tt.strategy})
		})
	}
}
