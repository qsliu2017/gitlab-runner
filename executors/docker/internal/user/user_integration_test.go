package user

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func createContainerSetup(t *testing.T, image string, fn func(docker.Client, string)) {
	helpers.SkipIntegrationTests(t, "docker", "info")
	if runtime.GOOS == "windows" {
		t.Skip("Skipping unix test on windows")
	}

	client, err := docker.New(docker.Credentials{}, "")
	require.NoError(t, err, "should be able to connect to docker")
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = client.ImagePullBlocking(ctx, image, types.ImagePullOptions{})
	require.NoError(t, err)

	container, err := client.ContainerCreate(ctx, &container.Config{Image: image}, nil, nil, "")
	require.NoError(t, err)
	defer func() { _ = client.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{Force: true}) }()

	fn(client, container.ID)
}

func TestLookupUser(t *testing.T) {
	tests := []struct {
		input       string
		uid, gid    int
		expectedErr error
	}{
		{"root", 0, 0, nil},
		{"bin", 1, 1, nil},
		{"adm", 3, 4, nil},
		{"shutdown", 6, 0, nil},
		{"missing", 0, 0, ErrNoMatchingEntries},
		{"root:1", 0, 1, nil},
		{"bin:2", 1, 2, nil},
		{"adm:3", 3, 3, nil},
		{"shutdown:4", 6, 4, nil},
		{"missing:5", 0, 0, ErrNoMatchingEntries},
		{"0:root", 0, 0, nil},
		{"1:bin", 1, 1, nil},
		{"3:adm", 3, 4, nil},
		{"5:missing", 0, 0, ErrNoMatchingEntries},
		{"0:0", 0, 0, nil},
		{"1:1", 1, 1, nil},
		{"3:4", 3, 4, nil},
	}

	createContainerSetup(t, common.TestAlpineImage, func(client docker.Client, containerID string) {
		for _, tc := range tests {
			t.Run(tc.input, func(t *testing.T) {
				uid, gid, err := LookupUser(context.Background(), client, containerID, tc.input)

				assert.True(t, errors.Is(err, tc.expectedErr), "expected: %#v, got: %#v", tc.expectedErr, err)
				assert.Equal(t, tc.uid, uid, "uid not equal")
				assert.Equal(t, tc.gid, gid, "gid not equal")
			})
		}
	})
}

func TestLookupUserNoPasswdGroup(t *testing.T) {
	tests := []struct {
		input       string
		uid, gid    int
		expectedErr error
	}{
		{"root", 0, 0, ErrCopyingFile},
		{"root:0", 0, 0, ErrCopyingFile},
		{"0:root", 0, 0, ErrCopyingFile},
		{"0:0", 0, 0, nil},
	}

	createContainerSetup(t, "hello-world", func(client docker.Client, containerID string) {
		for _, tc := range tests {
			t.Run(tc.input, func(t *testing.T) {
				uid, gid, err := LookupUser(context.Background(), client, containerID, tc.input)

				assert.True(t, errors.Is(err, tc.expectedErr), "expected: %#v, got: %#v", tc.expectedErr, err)
				assert.Equal(t, tc.uid, uid, "uid not equal")
				assert.Equal(t, tc.gid, gid, "gid not equal")
			})
		}
	})
}
