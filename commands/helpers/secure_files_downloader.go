package helpers

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type SecureFilesDownloaderCommand struct {
	retryHelper
	network common.Network

	DownloadURL       string `long:"url" description:"The file download URL"`
	Name              string `long:"name" description:"The file name"`
	Checksum          string `long:"checksum" description:"The file checksum"`
	ChecksumAlgorithm string `long:"checksum_algorithm" description:"The algorithm used to compute the checksum"`
	DownloadPath      string `long:"path" description:"Where to download the file"`
	Token             string `long:"token" description:"The CI JOB TOKEN"`
}

func (c *SecureFilesDownloaderCommand) Execute(cliContext *cli.Context) {
	log.SetRunnerFormatter()

	_, err := os.Getwd()
	if err != nil {
		logrus.Fatalln("Unable to get working directory")
	}
	if c.DownloadURL == "" {
		logrus.Warningln("Missing DownloadURL (--url)")
	}
	if c.Token == "" {
		logrus.Warningln("Missing runner credentials (--token)")
	}
	if c.Name == "" {
		logrus.Warningln("Missing file name (--name)")
	}
	if c.DownloadPath == "" {
		logrus.Warningln("Missing download path (--path)")
	}
	if c.DownloadPath == "" {
		logrus.Warningln("Missing download path (--download_path)")
	}
	if c.DownloadURL == "" || c.Token == "" || c.Name == "" || c.DownloadPath == "" {
		logrus.Fatalln("Incomplete arguments")
	}

	err = os.MkdirAll(c.DownloadPath, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		logrus.Fatalln(err)
	}

	filepath := c.DownloadPath + "/" + c.Name

	DownloadFile(c.DownloadURL, filepath, c.Token)
}

func DownloadFile(url string, filepath string, token string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	// resp, err := http.Get(url)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("JOB-TOKEN", token)
	resp, err := client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	common.RegisterCommand2(
		"secure-files-downloader",
		"download all secure files (internal)",
		&SecureFilesDownloaderCommand{
			network: network.NewGitLabClient(),
			retryHelper: retryHelper{
				Retry:     2,
				RetryTime: time.Second,
			},
		},
	)
}
