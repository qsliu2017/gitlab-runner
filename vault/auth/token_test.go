package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestToken_Authenticate(t *testing.T) {
	testToken := "test-token"

	tests := map[string]struct {
		tokenInfo     client.TokenInfo
		err           error
		expectedError string
	}{
		"TokenLookupSelf returns an error": {
			tokenInfo:     client.TokenInfo{},
			err:           errors.New("test-error"),
			expectedError: "couldn't self-lookup the token: test-error",
		},
		"TokenLookupSelf returns token info": {
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

			clientMock.On("SetToken", testToken).Once()
			clientMock.On("TokenLookupSelf").
				Return(test.tokenInfo, test.err).
				Once()

			cfg := config.VaultAuth{
				Token: &config.VaultTokenAuth{
					Token: testToken,
				},
			}

			a := NewToken()
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
