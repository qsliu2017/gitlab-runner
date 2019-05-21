package validator

import (
	"path"
	"strings"

	"github.com/docker/docker/api/types"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"
)

type Validator interface {
	IsAbs(dir string) bool
}

type validatorFactory func() Validator

var supportedOSTypes = map[string]validatorFactory{
	OSTypeLinux:   newLinuxValidator,
	OSTypeWindows: newWindowsValidator,
}

func New(info types.Info) (Validator, error) {
	factory, ok := supportedOSTypes[info.OSType]
	if !ok {
		return nil, errors.NewErrOSNotSupported(info.OSType)
	}
	return factory(), nil
}

func newLinuxValidator() Validator {
	return new(LinuxValidator)
}

func newWindowsValidator() Validator {
	return new(WindowsValidator)
}

type LinuxValidator struct{}

func (*LinuxValidator) IsAbs(dir string) bool {
	return path.IsAbs(dir)
}

type WindowsValidator struct{}

// Taken from Go source code: https://github.com/golang/go/blob/648c7b592a30b2280e8d23419224c657ab0a8332/src/os/path_windows.go#L42-L52
//
// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
func (*WindowsValidator) IsAbs(dir string) bool {
	if isReservedName(dir) {
		return true
	}
	l := volumeNameLen(dir)
	if l == 0 {
		return false
	}
	dir = dir[l:]
	if dir == "" {
		return false
	}
	return isSlash(dir[0])
}

// reservedNames lists reserved Windows names. Search for PRN in
// https://docs.microsoft.com/en-us/windows/desktop/fileio/naming-a-file
// for details.
var reservedNames = []string{
	"CON", "PRN", "AUX", "NUL",
	"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
	"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
}

// isReservedName returns true, if path is Windows reserved name.
// See reservedNames for the full list.
func isReservedName(path string) bool {
	if len(path) == 0 {
		return false
	}
	for _, reserved := range reservedNames {
		if strings.EqualFold(path, reserved) {
			return true
		}
	}
	return false
}

// volumeNameLen returns length of the leading volume name on Windows.
// It returns 0 elsewhere.
func volumeNameLen(path string) int {
	if len(path) < 2 {
		return 0
	}
	// with drive letter
	c := path[0]
	if path[1] == ':' && ('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z') {
		return 2
	}
	// is it UNC? https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
	if l := len(path); l >= 5 && isSlash(path[0]) && isSlash(path[1]) &&
		!isSlash(path[2]) && path[2] != '.' {
		// first, leading `\\` and next shouldn't be `\`. its server name.
		for n := 3; n < l-1; n++ {
			// second, next '\' shouldn't be repeated.
			if isSlash(path[n]) {
				n++
				// third, following something characters. its share name.
				if !isSlash(path[n]) {
					if path[n] == '.' {
						break
					}
					for ; n < l; n++ {
						if isSlash(path[n]) {
							break
						}
					}
					return n
				}
				break
			}
		}
	}
	return 0
}

func isSlash(c uint8) bool {
	return c == '\\' || c == '/'
}
