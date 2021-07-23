package network

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	updateIntervalHeader = "X-GitLab-Trace-Update-Interval"
	pingIntervalHeader   = "X-GitLab-Trace-Ping-Interval"
	remoteStateHeader    = "Job-Status"

	statusCanceling = "canceling"
	statusCanceled  = "canceled"
	statusFailed    = "failed"
)

type RemoteJobStateResponse struct {
	StatusCode           int
	RemoteState          string
	RemoteUpdateInterval time.Duration
	RemotePingInterval   time.Duration
}

func (r *RemoteJobStateResponse) IsFailed() bool {
	if r.RemoteState == statusCanceled || r.RemoteState == statusFailed {
		return true
	}

	if r.StatusCode == http.StatusForbidden {
		return true
	}

	return false
}

func (r *RemoteJobStateResponse) IsCanceled() bool {
	return r.RemoteState == statusCanceling
}

func NewRemoteJobStateResponse(response *http.Response, logger logrus.FieldLogger) *RemoteJobStateResponse {
	if response == nil {
		return &RemoteJobStateResponse{}
	}

	result := &RemoteJobStateResponse{
		StatusCode:           response.StatusCode,
		RemoteState:          response.Header.Get(remoteStateHeader),
		RemoteUpdateInterval: parseHeaderInterval(response, updateIntervalHeader, logger),
		RemotePingInterval:   parseHeaderInterval(response, pingIntervalHeader, logger),
	}

	return result
}

func parseHeaderInterval(r *http.Response, header string, logger logrus.FieldLogger) time.Duration {
	raw := r.Header.Get(header)
	if raw == "" {
		return 0
	}

	interval, err := strconv.Atoi(raw)
	if err != nil {
		logger.WithError(err).WithField("header-value", raw).Warningf("Failed to parse %q header", header)
		return 0
	}

	return time.Duration(interval) * time.Second
}
