// +build !integration

package network

import (
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestNewTracePatchResponse(t *testing.T) {
	type hdr map[string]string

	testCases := map[string]struct {
		header                       hdr
		expectedRemoteUpdateInterval time.Duration
		expectedRemotePingInterval   time.Duration
	}{
		"nil response": {
			header:                       nil,
			expectedRemoteUpdateInterval: 0,
			expectedRemotePingInterval:   0,
		},
		"no intervals": {
			header:                       hdr{},
			expectedRemoteUpdateInterval: 0,
			expectedRemotePingInterval:   0,
		},
		"invalid update interval": {
			header:                       hdr{updateIntervalHeader: "invalid"},
			expectedRemoteUpdateInterval: 0,
		},
		"negative update interval": {
			header:                       hdr{updateIntervalHeader: "-10"},
			expectedRemoteUpdateInterval: time.Duration(-10) * time.Second,
		},
		"valid update interval": {
			header:                       hdr{updateIntervalHeader: "10"},
			expectedRemoteUpdateInterval: time.Duration(10) * time.Second,
		},
		"invalid ping interval": {
			header:                     hdr{pingIntervalHeader: "invalid"},
			expectedRemotePingInterval: 0,
		},
		"negative ping interval": {
			header:                     hdr{pingIntervalHeader: "-10"},
			expectedRemotePingInterval: time.Duration(-10) * time.Second,
		},
		"valid ping interval": {
			header:                     hdr{pingIntervalHeader: "10"},
			expectedRemotePingInterval: time.Duration(10) * time.Second,
		},
		"invalid ping and update interval": {
			header:                       hdr{pingIntervalHeader: "invalid", updateIntervalHeader: "invalid"},
			expectedRemotePingInterval:   0,
			expectedRemoteUpdateInterval: 0,
		},
		"valid ping and update interval": {
			header:                       hdr{pingIntervalHeader: "5", updateIntervalHeader: "10"},
			expectedRemotePingInterval:   time.Duration(5) * time.Second,
			expectedRemoteUpdateInterval: time.Duration(10) * time.Second,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			log, _ := test.NewNullLogger()

			var r *http.Response
			if tc.header != nil {
				r = &http.Response{Header: make(http.Header)}
				for key, val := range tc.header {
					r.Header.Add(key, val)
				}
			}
			tpr := NewRemoteJobStateResponse(r, log)

			assert.NotNil(t, tpr)
			assert.IsType(t, &RemoteJobStateResponse{}, tpr)
			assert.Equal(t, tc.expectedRemoteUpdateInterval, tpr.RemoteUpdateInterval)
			assert.Equal(t, tc.expectedRemotePingInterval, tpr.RemotePingInterval)
		})
	}
}
