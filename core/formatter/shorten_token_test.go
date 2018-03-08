package formatter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
)

func TestShortenToken(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"short", "short"},
		{"veryverylongtoken", "veryvery"},
	}

	for _, test := range tests {
		actual := formatter.ShortenToken(test.in)
		assert.Equal(t, test.out, actual)
	}
}
