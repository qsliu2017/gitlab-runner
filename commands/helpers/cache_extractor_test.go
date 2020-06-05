package helpers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/klauspost/compress/zip"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const cacheExtractorArchive = "archive.zip"
const cacheExtractorTestArchivedFile = "archive_file"

func TestCacheExtractorValidArchive(t *testing.T) {
	file, err := os.Create(cacheExtractorArchive)
	assert.NoError(t, err)
	defer file.Close()
	defer os.Remove(file.Name())
	defer os.Remove(cacheExtractorTestArchivedFile)

	archive := zip.NewWriter(file)
	_, err = archive.Create(cacheExtractorTestArchivedFile)
	assert.NoError(t, err)

	archive.Close()

	_, err = os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)

	cmd := CacheExtractorCommand{
		File: cacheExtractorArchive,
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})

	_, err = os.Stat(cacheExtractorTestArchivedFile)
	assert.NoError(t, err)
}

func TestCacheExtractorForInvalidArchive(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	writeTestFile(t, cacheExtractorArchive)
	defer os.Remove(cacheExtractorArchive)

	cmd := CacheExtractorCommand{
		File: cacheExtractorArchive,
	}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheExtractorForIfNoFileDefined(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheExtractorForNotExistingFile(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{
		File: "/../../../test.zip",
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func testServeCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "408 Method not allowed", 408)
		return
	}
	if r.URL.Path != "/cache.zip" {
		if r.URL.Path == "/timeout" {
			time.Sleep(50 * time.Millisecond)
		}
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
	archive := zip.NewWriter(w)
	_, _ = archive.Create(cacheExtractorTestArchivedFile)
	archive.Close()
}

func TestCacheExtractorRemoteServerNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testServeCache))
	defer ts.Close()

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{
		File:    "non-existing-test.zip",
		URL:     ts.URL + "/invalid-file.zip",
		Timeout: 0,
	}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}

func TestCacheExtractorRemoteServerTimedOut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testServeCache))
	defer ts.Close()

	output := logrus.StandardLogger().Out
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(output)
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	cmd := CacheExtractorCommand{
		File: "non-existing-test.zip",
		URL:  ts.URL + "/timeout",
	}
	cmd.getClient().Timeout = 1 * time.Millisecond

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
	assert.Contains(t, buf.String(), "Client.Timeout")

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}

func TestCacheExtractorRemoteServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testServeCache))
	defer ts.Close()

	defer os.Remove(cacheExtractorArchive)
	defer os.Remove(cacheExtractorTestArchivedFile)
	os.Remove(cacheExtractorArchive)
	os.Remove(cacheExtractorTestArchivedFile)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{
		File:    cacheExtractorArchive,
		URL:     ts.URL + "/cache.zip",
		Timeout: 0,
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.NoError(t, err)

	err = os.Chtimes(cacheExtractorArchive, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	assert.NoError(t, err)

	assert.NotPanics(t, func() { cmd.Execute(nil) }, "archive is up to date")
}

func TestCacheExtractorRemoteServerFailOnInvalidServer(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	os.Remove(cacheExtractorArchive)
	cmd := CacheExtractorCommand{
		File:    cacheExtractorArchive,
		URL:     "http://localhost:65333/cache.zip",
		Timeout: 0,
	}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}
