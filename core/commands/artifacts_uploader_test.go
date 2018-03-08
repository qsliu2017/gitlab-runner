package commands

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/core/network"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func mockArtifactsUploaderCommand(t *testing.T, result interface{}, retry int) (cmd *ArtifactsUploaderCommand, client *network.MockArtifactsClient) {
	credentials := network.JobCredentials{
		ID:    1000,
		Token: "test",
		URL:   "test",
	}
	client = new(network.MockArtifactsClient)

	client.On("UploadRawArtifacts", credentials, mock.Anything, mock.Anything, mock.Anything).Times(retry + 1).Return(result)

	cmd = &ArtifactsUploaderCommand{
		JobCredentials: credentials,
		client:         client,
	}

	if retry > 0 {
		cmd.retryHelper = retryHelper{Retry: retry}
	}

	return cmd, client
}

func TestArtifactsUploaderRequirements(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	cmd := ArtifactsUploaderCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsUploaderTooLarge(t *testing.T) {
	cmd, client := mockArtifactsUploaderCommand(t, network.UploadTooLarge, 0)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	client.AssertExpectations(t)
}

func TestArtifactsUploaderForbidden(t *testing.T) {
	cmd, client := mockArtifactsUploaderCommand(t, network.UploadForbidden, 0)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	client.AssertExpectations(t)
}

func TestArtifactsUploaderRetry(t *testing.T) {
	cmd, client := mockArtifactsUploaderCommand(t, network.UploadFailed, 2)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	client.AssertExpectations(t)
}

func TestArtifactsUploaderSucceeded(t *testing.T) {
	tmpFile, err := ioutil.TempFile(".", "artifact")
	require.NoError(t, err)
	artifactsTestArchivedFile := tmpFile.Name()
	defer os.Remove(artifactsTestArchivedFile)
	tmpFile.Close()

	mockResultFn := func(config network.JobCredentials, reader io.Reader, baseName string, expireIn string) network.UploadState {
		var buffer bytes.Buffer
		io.Copy(&buffer, reader)
		archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
		if err != nil {
			logrus.Warningln(err)
			return network.UploadForbidden
		}

		if len(archive.File) != 1 || archive.File[0].Name != artifactsTestArchivedFile {
			logrus.Warningln("Invalid archive:", len(archive.File))
			return network.UploadForbidden
		}

		return network.UploadSucceeded
	}

	cmd, client := mockArtifactsUploaderCommand(t, mockResultFn, 0)
	cmd.fileArchiver = fileArchiver{
		Paths: []string{artifactsTestArchivedFile},
	}
	cmd.Execute(nil)

	client.AssertExpectations(t)
}
