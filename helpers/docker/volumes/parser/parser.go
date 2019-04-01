package parser

import (
	"github.com/docker/docker/api/types"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
)

type Parser interface {
	ParseVolume(spec string) (*Volume, error)
}

type parserFactory func() Parser

var supportedOSTypes = map[string]parserFactory{
	common.OSTypeLinux:   newLinuxParser,
	common.OSTypeWindows: newWindowsParser,
}

func New(info types.Info) (Parser, error) {
	factory, ok := supportedOSTypes[info.OSType]
	if !ok {
		return nil, errors.NewUnsupportedOSTypeError(info.OSType)
	}
	return factory(), nil
}
