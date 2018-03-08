package formatter_test

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
)

func newBuildLogger(testName string, buff *bytes.Buffer) formatter.BuildLogger {
	return formatter.NewBuildLogger(buff, false, logrus.WithField("test", testName))
}

func runOnHijackedLogrusOutput(t *testing.T, handler func(t *testing.T, output *bytes.Buffer)) {
	oldOutput := logrus.StandardLogger().Out
	defer func() { logrus.StandardLogger().Out = oldOutput }()

	buf := bytes.NewBuffer([]byte{})
	logrus.StandardLogger().Out = buf

	handler(t, buf)
}

func TestLogLineWithoutSecret(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		jt := &bytes.Buffer{}
		l := newBuildLogger("log-line-without-secret", jt)

		l.Errorln("Fatal: Get http://localhost/?id=123")
		assert.Contains(t, jt.String(), `Get http://localhost/?id=123`)
		assert.Contains(t, output.String(), `Get http://localhost/?id=123`)
	})
}

func TestLogLineWithSecret(t *testing.T) {
	runOnHijackedLogrusOutput(t, func(t *testing.T, output *bytes.Buffer) {
		jt := &bytes.Buffer{}
		l := newBuildLogger("log-line-with-secret", jt)

		l.Errorln("Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234")
		assert.Contains(t, jt.String(), `Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]`)
		assert.Contains(t, output.String(), `Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234`)
	})
}
