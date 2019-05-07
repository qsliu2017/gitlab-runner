package secret

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/vault/client"
	"gitlab.com/gitlab-org/gitlab-runner/vault/config"
)

func TestKV1_Read(t *testing.T) {
	testKey := &config.VaultSecretKey{
		Key:     "test-key",
		EnvName: "test-env",
	}
	testPath := "test/path"

	tests := map[string]struct {
		setupCliMock     func(mockClient *client.MockClient)
		setupBuilderMock func(mockClient *MockBuilder)
		expectedError    string
	}{
		"error while reading data": {
			setupCliMock: func(mockClient *client.MockClient) {
				mockClient.On("Read", testPath).
					Return(nil, errors.New("test-error")).
					Once()
			},
			expectedError: `couldn't read KV1 secret for "test/path": test-error`,
		},
		"missing data for key": {
			setupCliMock: func(mockClient *client.MockClient) {
				output := map[string]interface{}{
					"other-key": nil,
				}

				mockClient.On("Read", testPath).
					Return(output, nil).
					Once()
			},
			expectedError: `no data for key "test-key" for KV1 secret "test/path"`,
		},
		"error while building the secret": {
			setupCliMock: func(mockClient *client.MockClient) {
				output := map[string]interface{}{
					"test-key": "test-value",
				}

				mockClient.On("Read", testPath).
					Return(output, nil).
					Once()
			},
			setupBuilderMock: func(mockBuilder *MockBuilder) {
				mockBuilder.On("BuildSecret", testKey, "test-value").
					Return(errors.New("test-error")).
					Once()
			},
			expectedError: `couldn't build secret for "test/path"::"test-key": test-error`,
		},
		"valid secret build": {
			setupCliMock: func(mockClient *client.MockClient) {
				output := map[string]interface{}{
					"test-key": "test-value",
				}

				mockClient.On("Read", testPath).
					Return(output, nil).
					Once()
			},
			setupBuilderMock: func(mockBuilder *MockBuilder) {
				mockBuilder.On("BuildSecret", testKey, "test-value").
					Return(nil).
					Once()
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			cliMock := new(client.MockClient)
			defer cliMock.AssertExpectations(t)

			if test.setupCliMock != nil {
				test.setupCliMock(cliMock)
			}

			builderMock := new(MockBuilder)
			defer builderMock.AssertExpectations(t)

			if test.setupBuilderMock != nil {
				test.setupBuilderMock(builderMock)
			}

			s := new(KV1)
			err := s.Read(cliMock, builderMock, testPath, &config.VaultSecret{
				Keys: config.VaultSecretKeys{testKey},
			})

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}
