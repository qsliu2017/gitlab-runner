package helpers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const cacheArchiverArchive = "archive.zip"
const cacheArchiverTestArchivedFile = "archive_file"

func TestCacheArchiverIsUpToDate(t *testing.T) {
	writeTestFile(t, cacheArchiverTestArchivedFile)
	defer os.Remove(cacheArchiverTestArchivedFile)

	defer os.Remove(cacheArchiverArchive)
	cmd := CacheArchiverCommand{
		File: cacheArchiverArchive,
		fileArchiver: fileArchiver{
			Paths: []string{
				cacheArchiverTestArchivedFile,
			},
		},
	}
	cmd.Execute(nil)
	fi, _ := os.Stat(cacheArchiverArchive)
	cmd.Execute(nil)
	fi2, _ := os.Stat(cacheArchiverArchive)
	assert.Equal(t, fi.ModTime(), fi2.ModTime(), "archive is up to date")

	// We need to wait one second, since the FS doesn't save milliseconds
	time.Sleep(time.Second)

	err := os.Chtimes(cacheArchiverTestArchivedFile, time.Now(), time.Now())
	assert.NoError(t, err)

	cmd.Execute(nil)
	fi3, _ := os.Stat(cacheArchiverArchive)
	assert.NotEqual(t, fi.ModTime(), fi3.ModTime(), "archive should get updated")
}

// This test pattern is taken from https://github.com/stretchr/testify/issues/858#issuecomment-568461833
// which is itself taken from https://talks.golang.org/2014/testing.slide#23
// There are some gotcha's to using this pattern though. Notably that the invocation of the test
// isn't actually what gets evaluated.
//
// This pattern works by adding an environment variable matching the tests name, then forks
// the test process and runs just the single test in a new sub-process. If the environment var
// exists, it knows it's in the fork and runs the actual test. The exit code of the sub-process
// can then be evaluated.
//
// There are some unintuitive behaivours to this practice. For instance, if you want to check
// a log message as well as the exit status, this isn't possible - the log message you'd check
// for on a assert following an assert that checks the output of this isn't the same invocation.
//
// This also probably has an impact on the evaluation of code coverage, however I'm not certain on that.
func testOsExitsNonZero(t *testing.T, funcName string, testFunction func(*testing.T)) bool {
	if os.Getenv(funcName) == "1" {
		testFunction(t)
		return true
	}
	cmd := exec.Command(os.Args[0], "-test.run="+funcName)
	cmd.Env = append(os.Environ(), funcName+"=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return true
	}

	return false
}

func TestCacheArchiverForIfNoFileDefined(t *testing.T) {
	cmd := CacheArchiverCommand{}
	assert.True(t,
		testOsExitsNonZero(t, "TestCacheArchiverForIfNoFileDefined", func(t *testing.T) {
			cmd.Execute(nil)
		}))
}

func testCacheUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/cache.zip" {
		if r.URL.Path == "/timeout" {
			time.Sleep(50 * time.Millisecond)
		}
		http.NotFound(w, r)
		return
	}
}

func TestCacheArchiverRemoteServerNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer ts.Close()

	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File:    cacheExtractorArchive,
		URL:     ts.URL + "/invalid-file.zip",
		Timeout: 0,
	}

	assert.True(t,
		testOsExitsNonZero(t, "TestCacheArchiverRemoteServerNotFound", func(t *testing.T) {
			cmd.Execute(nil)
		}))
}

func TestCacheArchiverRemoteServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer ts.Close()

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File:    cacheExtractorArchive,
		URL:     ts.URL + "/cache.zip",
		Timeout: 0,
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheArchiverRemoteServerTimedOut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer ts.Close()

	output := logrus.StandardLogger().Out
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(output)
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File: cacheExtractorArchive,
		URL:  ts.URL + "/timeout",
	}
	cmd.getClient().Timeout = 1 * time.Millisecond

	assert.True(t,
		testOsExitsNonZero(t, "TestCacheArchiverRemoteServerTimedOut", func(t *testing.T) {
			cmd.Execute(nil)
		}))
	// assert.Contains(t, buf.String(), "Client.Timeout")
}

func TestCacheArchiverRemoteServerFailOnInvalidServer(t *testing.T) {
	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File:    cacheExtractorArchive,
		URL:     "http://localhost:65333/cache.zip",
		Timeout: 0,
	}

	assert.True(t,
		testOsExitsNonZero(t, "TestCacheArchiverRemoteServerTimedOut", func(t *testing.T) {
			cmd.Execute(nil)
		}))

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}
