package helpers

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type ReleaseUploaderCommand struct {
	common.JobCredentials
	fileArchiver
	retryHelper
	network common.Network

	Tag         string `long:"tag" description:"The tag of the release"`
	Description string `long:"description" description:"The description of the release"`
}

func (c *ReleaseUploaderCommand) createRelease() error {
	// Create the release
	options := common.ReleaseOptions{
		Tag:         c.Tag,
		Description: c.Description,
	}

	// Upload the data
	switch c.network.CreateRelease(c.JobCredentials, options) {
	case common.UploadSucceeded:
		return nil
	case common.UploadForbidden:
		return os.ErrPermission
	case common.UploadTooLarge:
		return errors.New("too large")
	case common.UploadFailed:
		return retryableErr{err: os.ErrInvalid}
	default:
		return os.ErrInvalid
	}
}

func (c *ReleaseUploaderCommand) Execute(*cli.Context) {
	fmt.Printf("release_uploader.go: Execute:")
	log.SetRunnerFormatter()

	if len(c.URL) == 0 || len(c.Token) == 0 {
		logrus.Fatalln("Missing runner credentials")
	}
	if c.ID <= 0 {
		logrus.Fatalln("Missing build ID")
	}

	// Enumerate files
	err := c.enumerate()
	if err != nil {
		logrus.Fatalln(err)
	}

	// If the upload fails, exit with a non-zero exit code to indicate an issue?
	err = c.doRetry(c.createRelease)
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	common.RegisterCommand2("release-uploader", "create a release entry (internal)", &ReleaseUploaderCommand{
		network: network.NewGitLabClient(),
		retryHelper: retryHelper{
			Retry:     2,
			RetryTime: time.Second,
		},
		Name: "release",
	})
}
