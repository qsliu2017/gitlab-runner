package referees

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var badNetworkRefereeConfigs = []*NetworkRefereeConfig{
	{
		ElasticsearchCloudID: "1234",
	},
	{
		ElasticsearchCloudID:  "1234",
		ElasticsearchUsername: "abcd",
	},
	{
		ElasticsearchCloudID:  "1234",
		ElasticsearchUsername: "abcd",
		ElasticsearchPassword: "efgh",
	},
	{
		ElasticsearchAddresses: []string{"192.168.1.1"},
	},
	{
		ElasticsearchAddresses: []string{"192.168.1.1"},
		ElasticsearchAPIKey:    "abcd",
	},
	{
		ElasticsearchUsername: "abcd",
		ElasticsearchPassword: "efgh",
	},
	{},
}

func TestValidateBadConfigs(t *testing.T) {
	for _, badNetworkRefereeConfig := range badNetworkRefereeConfigs {
		err := validateConfig(badNetworkRefereeConfig)
		require.NotNil(t, err, badNetworkRefereeConfig)
		var missingCfgErr *missingConfigError
		assert.True(t, errors.As(err, &missingCfgErr), "expected err to be type of missingConfigError")
	}
}

func TestNewNetworkRefereeBadConfig(t *testing.T) {
	logger := logrus.WithField("test", 1)
	networkReferee := newNetworkReferee(&BaseReferee{hostname: "runner-1234", logger: logger}, &Config{Network: &NetworkRefereeConfig{}})
	require.Nil(t, networkReferee)
}

func TestNewNetworkReferee(t *testing.T) {
	mockExecutor := new(interface{})
	networkReferee := newTestNetworkReferee(t, mockExecutor, newTestCloudNetworkRefereeConfig())

	assert.Equal(t, "network_referee.json", networkReferee.ArtifactBaseName())
	assert.Equal(t, "network_referee", networkReferee.ArtifactType())
	assert.Equal(t, "gzip", networkReferee.ArtifactFormat())

	mockExecutor = new(interface{})
	networkReferee = newTestNetworkReferee(t, mockExecutor, newTestAddressesNetworkRefereeConfig())
}

func newTestNetworkReferee(t *testing.T, executor interface{}, config *Config) *NetworkReferee {
	logger := logrus.WithField("test", 1)
	networkReferee, ok := newNetworkReferee(&BaseReferee{hostname: "runner-1234", logger: logger}, config).(*NetworkReferee)
	require.True(t, ok, "Not dealing with network referee")
	return networkReferee
}

func newTestCloudNetworkRefereeConfig() *Config {
	return &Config{
		Network: &NetworkRefereeConfig{
			ElasticsearchCloudID:  "test:dXMtZWFzdC0xLmF3cy5mb3VuZC5pbyRjZWM2ZjI2MWE3NGJmMjRjZTMzYmI4ODExYjg0Mjk0ZiRjNmMyY2E2ZDA0MjI0OWFmMGNjN2Q3YTllOTYyNTc0Mw==",
			ElasticsearchUsername: "abcd",
			ElasticsearchPassword: "efgh",
			ElasticsearchIndex:    "test",
		},
	}
}

func newTestAddressesNetworkRefereeConfig() *Config {
	return &Config{
		Network: &NetworkRefereeConfig{
			ElasticsearchAddresses: []string{"192.168.1.1"},
			ElasticsearchAPIKey:    "abcd",
			ElasticsearchIndex:     "test",
		},
	}
}

func TestNetworkRefereeExecute(t *testing.T) {
	mockExecutor := new(interface{})
	networkReferee := newTestNetworkReferee(t, mockExecutor, newTestCloudNetworkRefereeConfig())

	response := `{"success":"true"}`
	mockTransport := new(mockTransport)
	defer mockTransport.AssertExpectations(t)
	mockTransport.On("Perform", mock.Anything).Return(func(r *http.Request) *http.Response {
		defer r.Body.Close()
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(r.Body)
		assert.NoError(t, err)
		actual := buf.String()

		assert.Contains(t, actual, `{"query":{"bool":{"must":[{"match":{"fields.hostname":"runner-1234"}},{"range":{"@timestamp":{"gte":"2014-07-16T20:55:46Z","lte":"2014-07-16T20:57:26Z"}}}]}}}`)

		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.0",
			ProtoMajor: 1,
			ProtoMinor: 0,
			Body:       ioutil.NopCloser(strings.NewReader(response)),
		}
	}, nil)

	mockClient := &elasticsearch.Client{Transport: mockTransport, API: esapi.New(mockTransport)}
	networkReferee.client = mockClient

	startTime := time.Unix(1405544146, 0)
	endTime := time.Unix(1405544246, 0)
	body, err := networkReferee.Execute(context.Background(), startTime, endTime)
	require.NotNil(t, body)
	require.Nil(t, err)
	assert.Equal(t, response, string(body))
}

func TestNetworkRefereeExecuteError(t *testing.T) {
	testErr := errors.New("test error")
	mockExecutor := new(interface{})
	networkReferee := newTestNetworkReferee(t, mockExecutor, newTestCloudNetworkRefereeConfig())

	mockTransport := new(mockTransport)
	defer mockTransport.AssertExpectations(t)
	mockTransport.On("Perform", mock.Anything).Return(nil, testErr)

	// set mock client in network referee
	mockClient := &elasticsearch.Client{Transport: mockTransport, API: esapi.New(mockTransport)}
	networkReferee.client = mockClient

	startTime := time.Unix(1405544146, 0)
	endTime := time.Unix(1405544246, 0)
	result, err := networkReferee.Execute(context.Background(), startTime, endTime)
	assert.True(t, errors.Is(err, testErr), "expected error from Perform")
	assert.Nil(t, result)
}

func TestNetworkRefereeExecuteInvalidResponse(t *testing.T) {
	mockExecutor := new(interface{})
	networkReferee := newTestNetworkReferee(t, mockExecutor, newTestCloudNetworkRefereeConfig())

	mockTransport := new(mockTransport)
	defer mockTransport.AssertExpectations(t)
	mockTransport.On("Perform", mock.Anything).Return(
		&http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       ioutil.NopCloser(strings.NewReader("empty")),
		},
		nil,
	)

	// set mock client in network referee
	mockClient := &elasticsearch.Client{Transport: mockTransport, API: esapi.New(mockTransport)}
	networkReferee.client = mockClient

	networkReferee.queryTimeout = time.Second

	startTime := time.Unix(1405544146, 0)
	endTime := time.Unix(1405544246, 0)
	result, err := networkReferee.Execute(context.Background(), startTime, endTime)
	var badResponseErr *badResponseError
	assert.True(t, errors.As(err, &badResponseErr), "expected %T, got %T", badResponseErr, err)
	assert.Nil(t, result)
	assert.Equal(t, badResponseErr.status, "500 Internal Server Error")
}
