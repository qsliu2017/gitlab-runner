package vault

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestVault_Connect(t *testing.T) {
	tests := map[string]struct {
		clientInitError error
		setupClientMock func() *client.MockClient
		expectedError   string
	}{
		"error on client initialization": {
			setupClientMock: func() *client.MockClient {
				return nil
			},
			clientInitError: errors.New("test-error"),
			expectedError:   "couldn't connect Vault client to the Vault server: couldn't initialize Vault client: test-error",
		},
		"server not ready with error": {
			setupClientMock: func() *client.MockClient {
				mockClient := new(client.MockClient)
				mockClient.On("IsServerReady").
					Return(client.VaultServerReadyResp{State: false, Err: errors.New("test-error")}).
					Once()

				return mockClient
			},
			expectedError: "Vault server is not ready to receive connections: test-error",
		},
		"server not ready without error": {
			setupClientMock: func() *client.MockClient {
				mockClient := new(client.MockClient)
				mockClient.On("IsServerReady").
					Return(client.VaultServerReadyResp{State: false, Err: nil}).
					Once()

				return mockClient
			},
			expectedError: "Vault server is not ready to receive connections",
		},
		"connected to Vault properly": {
			setupClientMock: func() *client.MockClient {
				mockClient := new(client.MockClient)
				mockClient.On("IsServerReady").
					Return(client.VaultServerReadyResp{State: true, Err: nil}).
					Once()

				return mockClient
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			mockClient := test.setupClientMock()
			if mockClient != nil {
				defer mockClient.AssertExpectations(t)
			}

			oldNewClient := newClient
			defer func() {
				newClient = oldNewClient
			}()
			newClient = func(_ config.VaultServer) (client.Client, error) {
				return mockClient, test.clientInitError
			}

			v := New()
			err := v.Connect(config.VaultServer{})

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVault_DobuleConnect(t *testing.T) {
	mockClient := new(client.MockClient)
	mockClient.On("IsServerReady").
		Return(client.VaultServerReadyResp{State: true, Err: nil}).
		Twice()

	newClientCalls := 0
	oldNewClient := newClient
	defer func() {
		newClient = oldNewClient
	}()
	newClient = func(_ config.VaultServer) (client.Client, error) {
		newClientCalls++
		return mockClient, nil
	}

	v := New()

	err := v.Connect(config.VaultServer{})
	require.NoError(t, err)
	err = v.Connect(config.VaultServer{})
	require.NoError(t, err)

	assert.Equal(t, 1, newClientCalls)
}

func TestVault_Authenticate(t *testing.T) {
	userpassAuth := &config.VaultUserpassAuth{}
	tlsAuth := &config.VaultTLSAuth{}

	tests := map[string]struct {
		auth                   config.VaultAuth
		setupAuthenticatorMock func() *MockAuthenticator
		expectedError          string
	}{
		"missing authenticator factory": {
			auth: config.VaultAuth{
				Token: &config.VaultTokenAuth{},
			},
			setupAuthenticatorMock: func() *MockAuthenticator {
				return nil
			},
			expectedError: `couldn't create authenticator: authenticator factory for "*config.VaultTokenAuth" authentication method is unknown`,
		},
		"error on authentication": {
			auth: config.VaultAuth{
				Userpass: userpassAuth,
			},
			setupAuthenticatorMock: func() *MockAuthenticator {
				auth := config.VaultAuth{
					Userpass: userpassAuth,
				}

				authenticatorMock := new(MockAuthenticator)
				authenticatorMock.On("Authenticate", mock.Anything, auth).
					Return(client.TokenInfo{}, errors.New("test-error")).
					Once()

				return authenticatorMock
			},
			expectedError: `couldn't authenticate against Vault server: test-error`,
		},
		"authenticated properly": {
			auth: config.VaultAuth{
				Userpass: userpassAuth,
			},
			setupAuthenticatorMock: func() *MockAuthenticator {
				auth := config.VaultAuth{
					Userpass: userpassAuth,
				}

				authenticatorMock := new(MockAuthenticator)
				authenticatorMock.On("Authenticate", mock.Anything, auth).
					Return(client.TokenInfo{Token: "some-token"}, nil).
					Once()

				return authenticatorMock
			},
		},
		"with multiple defined authentications chooses the first one from struct definition": {
			auth: config.VaultAuth{
				TLS:      tlsAuth,
				Userpass: userpassAuth,
			},
			setupAuthenticatorMock: func() *MockAuthenticator {
				auth := config.VaultAuth{
					TLS:      tlsAuth,
					Userpass: userpassAuth,
				}

				authenticatorMock := new(MockAuthenticator)
				authenticatorMock.On("Authenticate", mock.Anything, auth).
					Return(client.TokenInfo{Token: "some-token"}, nil).
					Once()

				return authenticatorMock
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			authenticatorMock := test.setupAuthenticatorMock()
			if authenticatorMock != nil {
				defer authenticatorMock.AssertExpectations(t)
			}

			oldAuthenticatorFactories := authenticatorFactories
			defer func() {
				authenticatorFactories = oldAuthenticatorFactories
			}()
			authenticatorFactories = map[reflect.Type]AuthenticatorFactory{
				reflect.TypeOf(&config.VaultUserpassAuth{}): func() Authenticator { return authenticatorMock },
			}

			cli := new(client.MockClient)
			defer cli.AssertExpectations(t)

			if test.expectedError == "" {
				cli.On("SetToken", "some-token").Once()
			}

			v := new(vault)
			v.client = cli

			err := v.Authenticate(test.auth)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
