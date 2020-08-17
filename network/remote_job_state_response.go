package network

import (
	"net/http"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const remoteStateHeader = "Job-Status"

type RemoteJobStateResponse struct {
	StatusCode  int
	RemoteState common.JobState
}

func (r *RemoteJobStateResponse) IsFailed() bool {
	if r.RemoteState == common.Canceled || r.RemoteState == common.Failed {
		return true
	}

	if r.StatusCode == http.StatusForbidden {
		return true
	}

	return false
}

func (r *RemoteJobStateResponse) IsCanceled() bool {
	return r.RemoteState == common.Canceling
}

func NewRemoteJobStateResponse(response *http.Response) *RemoteJobStateResponse {
	if response == nil {
		return &RemoteJobStateResponse{}
	}

	return &RemoteJobStateResponse{
		StatusCode:  response.StatusCode,
		RemoteState: common.JobState(response.Header.Get(remoteStateHeader)),
	}
}
