package user

// tests in this package are loosely based on those found in:
// https://golang.org/src/os/user/lookup_unix_test.go
//
// runc parses lines slightly differently to go/libc. For example, +/- prefixed
// names are allowed by runc. We match this behaviour. For "correctly" formatted
// lines, the behaviour is identical.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

const passwdFile = `   # Example user file
root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
bin:x:2:3:bin:/bin:/usr/sbin/nologin
     indented:x:3:3:indented:/dev:/usr/sbin/nologin
sync:x:4:65534:sync:/bin:/bin/sync
negative:x:-5:60:games:/usr/games:/usr/sbin/nologin
man:x:6:12:man:/var/cache/man:/usr/sbin/nologin
allfields:x:6:12:mansplit,man2,man3,man4:/home/allfields:/usr/sbin/nologin
+plussign:x:8:10:man:/var/cache/man:/usr/sbin/nologin
-minussign:x:9:10:man:/var/cache/man:/usr/sbin/nologin
malformed:x:27:12 # more:colons:after:comment
struid:x:notanumber:12 # more:colons:after:comment
# commented:x:28:12:commented:/var/cache/man:/usr/sbin/nologin
      # commentindented:x:29:12:commentindented:/var/cache/man:/usr/sbin/nologin
struid2:x:30:badgid:struid2name:/home/struid:/usr/sbin/nologin
`

const groupFile = `# See the opendirectoryd(8) man page for additional
# information about Open Directory.
##
nobody:*:-2:
nogroup:*:-1:
wheel:*:0:root
emptyid:*::root
invalidgid:*:notanumber:root
+plussign:*:20:root
-minussign:*:21:root
daemon:*:1:root
    indented:*:7:
# comment:*:4:found
     # comment:*:4:found
kmem:*:2:root
`

func TestParseUser(t *testing.T) {
	tests := []struct {
		input               string
		username, groupname string
		uid, gid            int
	}{
		{"root:root", "root", "root", 0, 0},
		{"root:1", "root", "", 0, 1},
		{"1:root", "", "root", 1, 0},
		{"1:1", "", "", 1, 1},
		{"123:456:789", "", "", 123, 456},
		{":123:456:", "", "", 0, 123},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			username, groupname, uid, gid := parseUser(tc.input)
			assert.Equal(t, tc.username, username)
			assert.Equal(t, tc.groupname, groupname)
			assert.Equal(t, tc.uid, uid)
			assert.Equal(t, tc.gid, gid)
		})
	}
}

func TestParsePasswd(t *testing.T) {
	tests := []struct {
		name        string
		uid, gid    int
		expectedErr error
	}{
		{"negative", -5, 60, nil},
		{"bin", 2, 3, nil},
		{"notinthefile", 0, 0, ErrNoMatchingEntries},
		{"indented", 3, 3, nil},
		{"plussign", 0, 0, ErrNoMatchingEntries},
		{"+plussign", 8, 10, nil},
		{"minussign", 0, 0, ErrNoMatchingEntries},
		{"-minussign", 9, 10, nil},
		{"   indented", 0, 0, ErrNoMatchingEntries},
		{"commented", 0, 0, ErrNoMatchingEntries},
		{"commentindented", 0, 0, ErrNoMatchingEntries},
		{"malformed", 27, 0, nil},
		{"# commented", 0, 0, ErrNoMatchingEntries},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uid, gid, err := parseUIDGIDFromPasswd([]byte(passwdFile), tc.name)

			assert.True(t, errors.Is(err, tc.expectedErr), "expected: %#v, got: %#v", tc.expectedErr, err)
			assert.Equal(t, tc.uid, uid, "uid not equal")
			assert.Equal(t, tc.gid, gid, "gid not equal")
		})
	}
}

func TestParseGroup(t *testing.T) {
	tests := []struct {
		name        string
		gid         int
		expectedErr error
	}{
		{"nobody", -2, nil},
		{"kmem", 2, nil},
		{"notinthefile", 0, ErrNoMatchingEntries},
		{"comment", 0, ErrNoMatchingEntries},
		{"plussign", 0, ErrNoMatchingEntries},
		{"+plussign", 20, nil},
		{"-minussign", 21, nil},
		{"minussign", 0, ErrNoMatchingEntries},
		{"emptyid", 0, nil},
		{"invalidgid", 0, nil},
		{"indented", 7, nil},
		{"# comment", 0, ErrNoMatchingEntries},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gid, err := parseGIDFromGroup([]byte(groupFile), tc.name)

			assert.True(t, errors.Is(err, tc.expectedErr), "expected: %#v, got: %#v", tc.expectedErr, err)
			assert.Equal(t, tc.gid, gid, "gid not equal")
		})
	}
}
