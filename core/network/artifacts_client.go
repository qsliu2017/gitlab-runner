package network

import (
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
)

// ArtifactsClient interface handles artifacts upload and download operations
type ArtifactsClient interface {
	DownloadArtifacts(config JobCredentials, artifactsFile string) DownloadState
	UploadRawArtifacts(config JobCredentials, reader io.Reader, baseName string, expireIn string) UploadState
	UploadArtifacts(config JobCredentials, artifactsFile string) UploadState
}

type gitlabArtifactsClient struct{}

func (n *gitlabArtifactsClient) doRaw(credentials RequestCredentials, method, uri string, request io.Reader, requestType string, headers http.Header) (res *http.Response, err error) {
	c, err := NewClient(credentials)
	if err != nil {
		return nil, err
	}

	return c.Do(uri, method, request, requestType, headers)
}

func (n *gitlabArtifactsClient) createArtifactsForm(mpw *multipart.Writer, reader io.Reader, baseName string) error {
	wr, err := mpw.CreateFormFile("file", baseName)
	if err != nil {
		return err
	}

	_, err = io.Copy(wr, reader)
	if err != nil {
		return err
	}
	return nil
}

func (n *gitlabArtifactsClient) UploadRawArtifacts(config JobCredentials, reader io.Reader, baseName string, expireIn string) UploadState {
	pr, pw := io.Pipe()
	defer pr.Close()

	mpw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mpw.Close()
		err := n.createArtifactsForm(mpw, reader, baseName)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	query := url.Values{}
	if expireIn != "" {
		query.Set("expire_in", expireIn)
	}

	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	res, err := n.doRaw(&config, "POST", fmt.Sprintf("jobs/%d/artifacts?%s", config.ID, query.Encode()), pr, mpw.FormDataContentType(), headers)

	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": formatter.ShortenToken(config.Token),
	})

	if res != nil {
		log = log.WithField("responseStatus", res.Status)
	}

	if err != nil {
		log.WithError(err).Errorln("Uploading artifacts to coordinator...", "error")
		return UploadFailed
	}
	defer res.Body.Close()
	defer io.Copy(ioutil.Discard, res.Body)

	switch res.StatusCode {
	case http.StatusCreated:
		log.Println("Uploading artifacts to coordinator...", "ok")
		return UploadSucceeded
	case http.StatusForbidden:
		log.WithField("status", res.Status).Errorln("Uploading artifacts to coordinator...", "forbidden")
		return UploadForbidden
	case http.StatusRequestEntityTooLarge:
		log.WithField("status", res.Status).Errorln("Uploading artifacts to coordinator...", "too large archive")
		return UploadTooLarge
	default:
		log.WithField("status", res.Status).Warningln("Uploading artifacts to coordinator...", "failed")
		return UploadFailed
	}
}

func (n *gitlabArtifactsClient) UploadArtifacts(config JobCredentials, artifactsFile string) UploadState {
	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": formatter.ShortenToken(config.Token),
	})

	file, err := os.Open(artifactsFile)
	if err != nil {
		log.WithError(err).Errorln("Uploading artifacts to coordinator...", "error")
		return UploadFailed
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		log.WithError(err).Errorln("Uploading artifacts to coordinator...", "error")
		return UploadFailed
	}
	if fi.IsDir() {
		log.WithField("error", "cannot upload directories").Errorln("Uploading artifacts to coordinator...", "error")
		return UploadFailed
	}

	baseName := filepath.Base(artifactsFile)
	return n.UploadRawArtifacts(config, file, baseName, "")
}

func (n *gitlabArtifactsClient) DownloadArtifacts(config JobCredentials, artifactsFile string) DownloadState {
	headers := make(http.Header)
	headers.Set("JOB-TOKEN", config.Token)
	res, err := n.doRaw(&config, "GET", fmt.Sprintf("jobs/%d/artifacts", config.ID), nil, "", headers)

	log := logrus.WithFields(logrus.Fields{
		"id":    config.ID,
		"token": formatter.ShortenToken(config.Token),
	})

	if res != nil {
		log = log.WithField("responseStatus", res.Status)
	}

	if err != nil {
		log.Errorln("Downloading artifacts from coordinator...", "error", err.Error())
		return DownloadFailed
	}
	defer res.Body.Close()
	defer io.Copy(ioutil.Discard, res.Body)

	switch res.StatusCode {
	case http.StatusOK:
		file, err := os.Create(artifactsFile)
		if err == nil {
			defer file.Close()
			_, err = io.Copy(file, res.Body)
		}
		if err != nil {
			file.Close()
			os.Remove(file.Name())
			log.WithError(err).Errorln("Downloading artifacts from coordinator...", "error")
			return DownloadFailed
		}
		log.Println("Downloading artifacts from coordinator...", "ok")
		return DownloadSucceeded
	case http.StatusForbidden:
		log.WithField("status", res.Status).Errorln("Downloading artifacts from coordinator...", "forbidden")
		return DownloadForbidden
	case http.StatusNotFound:
		log.Errorln("Downloading artifacts from coordinator...", "not found")
		return DownloadNotFound
	default:
		log.WithField("status", res.Status).Warningln("Downloading artifacts from coordinator...", "failed")
		return DownloadFailed
	}
}

func NewArtifactsClient() ArtifactsClient {
	return &gitlabArtifactsClient{}
}
