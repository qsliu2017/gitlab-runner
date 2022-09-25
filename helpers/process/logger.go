package process

//go:generate mockery --inpackage --name Logger

import (
	"github.com/sirupsen/logrus"
)

type Logger interface {
	WithFields(fields logrus.Fields) Logger
	Warn(args ...interface{})
}
