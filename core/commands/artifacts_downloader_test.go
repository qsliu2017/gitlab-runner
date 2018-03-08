package commands

import (
	"archive/zip"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/core/network"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

var downloaderCredentials = network.JobCredentials{
	ID:    1000,
	Token: "test",
	URL:   "test",
}

func TestArtifactsDownloaderRequirements(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	cmd := ArtifactsDownloaderCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsDownloaderNotFound(t *testing.T) {
	client := new(network.MockArtifactsClient)

	client.On("DownloadArtifacts", downloaderCredentials, mock.AnythingOfType("string")).Once().Return(network.DownloadNotFound)

	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		client:         client,
	}

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	client.AssertExpectations(t)
}

func TestArtifactsDownloaderForbidden(t *testing.T) {
	client := new(network.MockArtifactsClient)
	client.On("DownloadArtifacts", downloaderCredentials, mock.AnythingOfType("string")).Once().Return(network.DownloadForbidden)

	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		client:         client,
	}

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	client.AssertExpectations(t)
}

func TestArtifactsDownloaderRetry(t *testing.T) {
	client := new(network.MockArtifactsClient)

	client.On("DownloadArtifacts", downloaderCredentials, mock.AnythingOfType("string")).Times(3).Return(network.DownloadFailed)

	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		client:         client,
		retryHelper: retryHelper{
			Retry: 2,
		},
	}

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	client.AssertExpectations(t)
}

func TestArtifactsDownloaderSucceeded(t *testing.T) {
	fileName := "a_file.txt"
	defer os.Remove(fileName)

	mockResultFn := func(_ network.JobCredentials, artifactsFile string) network.DownloadState {
		require.NotEmpty(t, artifactsFile)

		file, err := os.Create(artifactsFile)
		require.NoError(t, err)
		defer file.Close()

		archive := zip.NewWriter(file)
		archive.Create(fileName)
		archive.Close()

		return network.DownloadSucceeded
	}

	client := new(network.MockArtifactsClient)
	client.On("DownloadArtifacts", downloaderCredentials, mock.AnythingOfType("string")).Once().Return(mockResultFn)

	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		client:         client,
	}

	cmd.Execute(nil)
	client.AssertExpectations(t)
}
