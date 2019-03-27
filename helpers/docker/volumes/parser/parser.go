package parser

import (
	"github.com/docker/docker/api/types"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"
)

type Parser interface {
	ParseVolume(spec string) (*Volume, error)
}

type parserFactory func() Parser

var supportedOSTypes = map[string]parserFactory{
	OSTypeLinux:   newLinuxParser,
	OSTypeWindows: newWindowsParser,
}

func New(info types.Info) (Parser, error) {
	factory, ok := supportedOSTypes[info.OSType]
	if !ok {
		return nil, newUnsupportedOSTypeError(info.OSType)
	}
	return factory(), nil
}
