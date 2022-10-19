package tests

import (
	"context"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"testing"
	"time"
)

func GetRemoteBuildResponseFromServer(t *testing.T, config common.RunnerConfig) common.JobResponse {
	client := network.NewGitLabClient()

	var job *common.JobResponse
	for job == nil {
		t.Log("Requesting job from server....")

		job, _ = client.RequestJob(context.Background(), config, nil)
		time.Sleep(5 * time.Second)
	}

	return *job
}
