package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestNew(t *testing.T) {
	testURL := "https://localhost:1234"

	tests := map[string]struct {
		serverConfig  config.VaultServer
		expectedError string
	}{
		"fails TLS configuration": {
			serverConfig: config.VaultServer{
				URL:       testURL,
				TLSCAFile: "/tmp/not-existing",
			},
			expectedError: "couldn't prepare TLS configuration for the new Vault client: Error loading CA File: open /tmp/not-existing: no such file or directory",
		},
		"fails client initialization": {
			serverConfig: config.VaultServer{
				URL: ":",
			},
			expectedError: "couldn't create new Vault client: parse :: missing protocol scheme",
		},
		"creates client properly": {
			serverConfig: config.VaultServer{
				URL: "http://127.0.0.1:8200/",
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			cli, err := New(test.serverConfig)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.Nil(t, cli)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, cli)
		})
	}
}

func TestClient_IsServerReady(t *testing.T) {
	tests := map[string]struct {
		healthOutput   string
		expectedStatus bool
		expectedError  string
	}{
		"request returns an error": {
			healthOutput:   `abc`,
			expectedStatus: false,
			expectedError:  "invalid character 'a' looking for beginning of value",
		},
		"server is not initialized": {
			healthOutput:   `{"initialized":false, "sealed":true}`,
			expectedStatus: false,
		},
		"server is sealed": {
			healthOutput:   `{"initialized":true, "sealed":true}`,
			expectedStatus: false,
		},
		"server is ready": {
			healthOutput:   `{"initialized":true, "sealed":false}`,
			expectedStatus: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/sys/health" {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(test.healthOutput))
			}))
			defer server.Close()

			cli, err := New(config.VaultServer{
				URL: server.URL,
			})
			require.NoError(t, err)

			resp := cli.IsServerReady()
			assert.Equal(t, test.expectedStatus, resp.State)
			if test.expectedError != "" {
				assert.EqualError(t, resp.Err, test.expectedError)
			} else {
				assert.NoError(t, resp.Err)
			}
		})
	}
}

func TestClient_SetToken(t *testing.T) {
	c := new(client)
	c.c = new(api.Client)

	assert.Empty(t, c.c.Token())
	c.SetToken("test-token")
	assert.Equal(t, "test-token", c.c.Token())
}
