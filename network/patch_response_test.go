package network

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/network/internal/response"
)

func newResponseWithHeader(key string, value string) *http.Response {
	r := newResponse()
	r.Header.Add(key, value)

	return r
}

func newResponse() *http.Response {
	r := new(http.Response)
	r.Header = make(http.Header)
	r.Body = ioutil.NopCloser(new(bytes.Buffer))

	return r
}

func TestNewTracePatchResponse(t *testing.T) {
	tracePatchTestCases := map[string]struct {
		response                          *http.Response
		expectedRemoteTraceUpdateInterval time.Duration
	}{
		"nil response": {
			response:                          nil,
			expectedRemoteTraceUpdateInterval: time.Duration(emptyRemoteTraceUpdateInterval),
		},
		"no remote trace period in header": {
			response:                          newResponse(),
			expectedRemoteTraceUpdateInterval: time.Duration(emptyRemoteTraceUpdateInterval),
		},
		"invalid remote trace period in header": {
			response:                          newResponseWithHeader(traceUpdateIntervalHeader, "invalid"),
			expectedRemoteTraceUpdateInterval: time.Duration(emptyRemoteTraceUpdateInterval),
		},
		"negative remote trace period in header": {
			response:                          newResponseWithHeader(traceUpdateIntervalHeader, "-10"),
			expectedRemoteTraceUpdateInterval: time.Duration(-10) * time.Second,
		},
		"valid remote trace period in header": {
			response:                          newResponseWithHeader(traceUpdateIntervalHeader, "10"),
			expectedRemoteTraceUpdateInterval: time.Duration(10) * time.Second,
		},
	}

	for tn, tc := range tracePatchTestCases {
		t.Run(tn, func(t *testing.T) {
			log, _ := test.NewNullLogger()
			tpr := NewTracePatchResponse(response.New(tc.response), log)

			assert.NotNil(t, tpr)
			assert.IsType(t, &TracePatchResponse{}, tpr)
			assert.Equal(t, tc.expectedRemoteTraceUpdateInterval, tpr.RemoteTraceUpdateInterval)
		})
	}
}
