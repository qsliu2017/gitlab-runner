package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestTLS_Authenticate(t *testing.T) {
	testToken := "test-token"
	name := "some-role-name"
	tlsCertFile := "/path/to/tls.cert"
	tlsKeyFile := "/path/to/tls.key"

	tests := map[string]struct {
		path          string
		expectedPath  string
		tokenInfo     client.TokenInfo
		err           error
		expectedError string
	}{
		"TLSLogin returns an error": {
			expectedPath:  config.DefaultTLSAuthPath,
			tokenInfo:     client.TokenInfo{},
			err:           errors.New("test-error"),
			expectedError: "couldn't authenticate with TLS method: test-error",
		},
		"TLSLogin returns token info - path defined": {
			path:         "some-tls-path",
			expectedPath: "some-tls-path",
			tokenInfo: client.TokenInfo{
				Token: testToken,
				TTL:   10 * time.Second,
			},
			err:           nil,
			expectedError: "",
		},
		"TLSLogin returns token info - path undefined": {
			expectedPath: config.DefaultTLSAuthPath,
			tokenInfo: client.TokenInfo{
				Token: testToken,
				TTL:   10 * time.Second,
			},
			err:           nil,
			expectedError: "",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			clientMock := new(client.MockClient)
			defer clientMock.AssertExpectations(t)

			clientMock.On("TLSLogin", test.expectedPath, name, tlsCertFile, tlsKeyFile).
				Return(test.tokenInfo, test.err).
				Once()

			cfg := config.VaultAuth{
				TLS: &config.VaultTLSAuth{
					Path:        test.path,
					Name:        name,
					TLSCertFile: tlsCertFile,
					TLSKeyFile:  tlsKeyFile,
				},
			}

			a := NewTLS()
			tokenInf, err := a.Authenticate(clientMock, cfg)

			assert.Equal(t, test.tokenInfo, tokenInf)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
		})
	}
}
