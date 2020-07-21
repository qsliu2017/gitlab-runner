package user

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

var (
	ErrNoMatchingEntries = errors.New("no matching entries")
	ErrCopyingFile       = errors.New("copying failed")
)

func LookupUser(ctx context.Context, c docker.Client, containerID, user string) (int, int, error) {
	username, groupname, uid, gid := parseUser(user)

	// resolve username to uid, gid
	if username != "" {
		data, err := copyFromContainer(ctx, c, containerID, "/etc/passwd")
		if err != nil {
			return 0, 0, fmt.Errorf("%v: %w", err, ErrCopyingFile)
		}

		var new int
		uid, new, err = parseUIDGIDFromPasswd(data, username)
		if err != nil {
			return 0, 0, fmt.Errorf("unable to find user %s: %w", username, err)
		}
		if groupname == "" && gid == 0 {
			gid = new
		}
	}

	// resolve groupname to gid
	if groupname != "" {
		data, err := copyFromContainer(ctx, c, containerID, "/etc/group")
		if err != nil {
			return 0, 0, fmt.Errorf("%v: %w", err, ErrCopyingFile)
		}

		gid, err = parseGIDFromGroup(data, groupname)
		if err != nil {
			return 0, 0, fmt.Errorf("unable to find group %s: %w", groupname, err)
		}
	}

	return uid, gid, nil
}

func copyFromContainer(ctx context.Context, c docker.Client, containerID, pathname string) ([]byte, error) {
	rc, _, err := c.CopyFromContainer(ctx, containerID, pathname)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	r := tar.NewReader(io.LimitReader(rc, 1024*1024))
	for {
		header, err := r.Next()
		switch {
		case err != nil:
			return nil, err

		case header.Name == path.Base(pathname):
			return ioutil.ReadAll(r)
		}
	}
}

func parseUser(user string) (username, groupname string, uid, gid int) {
	var err error
	co := strings.Split(user, ":")
	if len(co) > 0 {
		uid, err = strconv.Atoi(co[0])
		if err != nil {
			username = co[0]
		}
	}

	if len(co) > 1 {
		gid, err = strconv.Atoi(co[1])
		if err != nil {
			groupname = co[1]
		}
	}

	return username, groupname, uid, gid
}

func parseUIDGIDFromPasswd(data []byte, user string) (int, int, error) {
	uidGidMap := map[int]int{2: 0, 3: 0}

	return uidGidMap[2], uidGidMap[3], parsePasswdGroupIDs(data, user, 4, uidGidMap)
}

func parseGIDFromGroup(data []byte, group string) (int, error) {
	gidMap := map[int]int{2: 0}

	return gidMap[2], parsePasswdGroupIDs(data, group, 3, gidMap)
}

func parsePasswdGroupIDs(data []byte, identifier string, count int, ids map[int]int) error {
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		co := strings.SplitN(string(bytes.TrimSpace(line)), ":", 5)
		if len(co) < count || strings.HasPrefix(co[0], "#") || co[0] != identifier {
			continue
		}

		for idx := range ids {
			id, err := strconv.Atoi(co[idx])
			if err == nil {
				ids[idx] = id
			}
		}

		return nil
	}

	return ErrNoMatchingEntries
}
