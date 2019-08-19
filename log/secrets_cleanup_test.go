package log

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSecretsCleanupHook(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		fields   logrus.Fields
		expected string
	}{
		{
			name:     "With Secrets",
			message:  "Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234",
			expected: "Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]",
		},
		{
			name:    "Secrets in fields",
			message: "Something happened",
			fields: map[string]interface{}{
				"error": "Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234",
			},
			expected: `error="Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]"`,
		},
		{
			name:     "No Secrets",
			message:  "Fatal: Get http://localhost/?id=123",
			expected: "Fatal: Get http://localhost/?id=123",
		},
		{
			name:    "It drops fields that can't be converted to string",
			message: "Something happened",
			fields: map[string]interface{}{
				"error": SecretsCleanupHook{},
			},
			expected: "error=\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}

			logger := logrus.New()
			logger.Out = buffer
			AddSecretsCleanupLogHook(logger)

			logger.WithFields(test.fields).Errorln(test.message)

			assert.Contains(t, buffer.String(), test.expected)
		})
	}
}

func BenchmarkSecretsCleanupHook_Fire(b *testing.B) {
	for n := 0; n < b.N; n++ {
		entry := &logrus.Entry{
			Data: map[string]interface{}{
				"error": "Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234",
			},
			Message: "Something happened",
		}

		hook := SecretsCleanupHook{}

		_ = hook.Fire(entry)
	}
}
