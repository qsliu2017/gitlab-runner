package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestUserpass_Authenticate(t *testing.T) {
	testToken := "test-token"
	username := "user"
	password := "pass"

	tests := map[string]struct {
		path          string
		expectedPath  string
		tokenInfo     client.TokenInfo
		err           error
		expectedError string
	}{
		"UserpassLogin returns an error": {
			expectedPath:  config.DefaultUserpassAuthPath,
			tokenInfo:     client.TokenInfo{},
			err:           errors.New("test-error"),
			expectedError: "couldn't authenticate with userpass method: test-error",
		},
		"UserpassLogin returns token info - path defined": {
			path:         "some-userpass-path",
			expectedPath: "some-userpass-path",
			tokenInfo: client.TokenInfo{
				Token: testToken,
				TTL:   10 * time.Second,
			},
			err:           nil,
			expectedError: "",
		},
		"UserpassLogin returns token info - path undefined": {
			expectedPath: config.DefaultUserpassAuthPath,
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

			clientMock.On("UserpassLogin", test.expectedPath, username, password).
				Return(test.tokenInfo, test.err).
				Once()

			cfg := config.VaultAuth{
				Userpass: &config.VaultUserpassAuth{
					Path:     test.path,
					Username: username,
					Password: password,
				},
			}

			a := NewUserpass()
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
