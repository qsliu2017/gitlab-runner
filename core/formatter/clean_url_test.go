package formatter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/core/formatter"
)

func TestRemovingAllSensitiveData(t *testing.T) {
	url := formatter.CleanURL("https://user:password@gitlab.com/gitlab?key=value#fragment")
	assert.Equal(t, "https://gitlab.com/gitlab", url)
}

func TestInvalidURL(t *testing.T) {
	assert.Empty(t, formatter.CleanURL("://invalid URL"))
}
