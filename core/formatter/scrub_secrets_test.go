package formatter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
)

func TestScrubSecrets(t *testing.T) {
	examples := []struct {
		input  string
		output string
	}{
		{input: "Get http://localhost/?id=123", output: "Get http://localhost/?id=123"},
		{input: "Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234", output: "Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]"},
	}

	for _, example := range examples {
		assert.Equal(t, example.output, formatter.ScrubSecrets(example.input))
	}
}
