package commands

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/core/archives"
	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
	"gitlab.com/gitlab-org/gitlab-runner/core/network"
)

type ArtifactsDownloaderCommand struct {
	network.JobCredentials
	retryHelper
	client network.ArtifactsClient
}

func (c *ArtifactsDownloaderCommand) download(file string) (bool, error) {
	switch c.client.DownloadArtifacts(c.JobCredentials, file) {
	case network.DownloadSucceeded:
		return false, nil
	case network.DownloadNotFound:
		return false, os.ErrNotExist
	case network.DownloadForbidden:
		return false, os.ErrPermission
	case network.DownloadFailed:
		return true, os.ErrInvalid
	default:
		return false, os.ErrInvalid
	}
}

func (c *ArtifactsDownloaderCommand) Execute(context *cli.Context) {
	formatter.SetRunnerFormatter()

	if len(c.URL) == 0 || len(c.Token) == 0 {
		logrus.Fatalln("Missing runner credentials")
	}
	if c.ID <= 0 {
		logrus.Fatalln("Missing build ID")
	}

	// Create temporary file
	file, err := ioutil.TempFile("", "artifacts")
	if err != nil {
		logrus.Fatalln(err)
	}
	file.Close()
	defer os.Remove(file.Name())

	// Download artifacts file
	err = c.doRetry(func() (bool, error) {
		return c.download(file.Name())
	})
	if err != nil {
		logrus.Fatalln(err)
	}

	// Extract artifacts file
	err = archives.ExtractZipFile(file.Name())
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	RegisterCommand2("artifacts-downloader", "download and extract build artifacts (internal)", &ArtifactsDownloaderCommand{
		client: network.NewArtifactsClient(),
		retryHelper: retryHelper{
			Retry:     2,
			RetryTime: time.Second,
		},
	})
}
