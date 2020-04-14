package network

import (
	"net/http"
)

const (
	remoteStateHeader = "Job-Status"

	stateCanceled           = "canceled"
	stateGracefullyCanceled = "gracefully-canceled"
	stateFailed             = "failed"
)

type RemoteJobStateResponse struct {
	StatusCode  int
	RemoteState string
}

func (r *RemoteJobStateResponse) IsAborted() bool {
	if r.RemoteState == stateCanceled || r.RemoteState == stateFailed {
		return true
	}

	if r.StatusCode == http.StatusForbidden {
		return true
	}

	return false
}

func (r *RemoteJobStateResponse) IsGracefullyCanceled() bool {
	return r.RemoteState == stateGracefullyCanceled
}

func NewRemoteJobStateResponse(response *http.Response) *RemoteJobStateResponse {
	if response == nil {
		return &RemoteJobStateResponse{}
	}

	return &RemoteJobStateResponse{
		StatusCode:  response.StatusCode,
		RemoteState: response.Header.Get(remoteStateHeader),
	}
}
