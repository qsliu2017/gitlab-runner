package network_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/core/network"
)

func testArtifactsUploadHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/jobs/10/artifacts" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get("JOB-TOKEN") != "token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(file)
	assert.NoError(t, err)

	if string(body) != "content" {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

func TestArtifactsUpload(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testArtifactsUploadHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	config := network.JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "token",
	}
	invalidToken := network.JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "invalid-token",
	}

	tempFile, err := ioutil.TempFile("", "artifacts")
	assert.NoError(t, err)
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	c := network.NewArtifactsClient()

	fmt.Fprint(tempFile, "content")
	state := c.UploadArtifacts(config, tempFile.Name())
	assert.Equal(t, network.UploadSucceeded, state, "Artifacts should be uploaded")

	fmt.Fprint(tempFile, "too large")
	state = c.UploadArtifacts(config, tempFile.Name())
	assert.Equal(t, network.UploadTooLarge, state, "Artifacts should be not uploaded, because of too large archive")

	state = c.UploadArtifacts(config, "not/existing/file")
	assert.Equal(t, network.UploadFailed, state, "Artifacts should fail to be uploaded")

	state = c.UploadArtifacts(invalidToken, tempFile.Name())
	assert.Equal(t, network.UploadForbidden, state, "Artifacts should be rejected if invalid token")
}

func testArtifactsDownloadHandler(w http.ResponseWriter, r *http.Request, t *testing.T) {
	if r.URL.Path != "/api/v4/jobs/10/artifacts" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	if r.Header.Get("JOB-TOKEN") != "token" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(bytes.NewBufferString("Test artifact file content").Bytes())
}

func TestArtifactsDownload(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		testArtifactsDownloadHandler(w, r, t)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	credentials := network.JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "token",
	}
	invalidTokenCredentials := network.JobCredentials{
		ID:    10,
		URL:   s.URL,
		Token: "invalid-token",
	}
	fileNotFoundTokenCredentials := network.JobCredentials{
		ID:    11,
		URL:   s.URL,
		Token: "token",
	}

	c := network.NewArtifactsClient()

	tempDir, err := ioutil.TempDir("", "artifacts")
	assert.NoError(t, err)

	artifactsFileName := filepath.Join(tempDir, "downloaded-artifact")
	defer os.Remove(artifactsFileName)

	state := c.DownloadArtifacts(credentials, artifactsFileName)
	assert.Equal(t, network.DownloadSucceeded, state, "Artifacts should be downloaded")

	state = c.DownloadArtifacts(invalidTokenCredentials, artifactsFileName)
	assert.Equal(t, network.DownloadForbidden, state, "Artifacts should be not downloaded if invalid token is used")

	state = c.DownloadArtifacts(fileNotFoundTokenCredentials, artifactsFileName)
	assert.Equal(t, network.DownloadNotFound, state, "Artifacts should be bit downloaded if it's not found")
}
