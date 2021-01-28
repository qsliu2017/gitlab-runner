package retry

import (
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type Retryable interface {
	Run() error
	ShouldRetry(tries int, err error) bool
}

type Retry interface {
	Run() error
}

func WithLogrus(retryable Retryable, log *logrus.Entry) Retryable {
	return NewRetryableDecorator(retryable.Run, func(tries int, err error) bool {
		shouldRetry := retryable.ShouldRetry(tries, err)
		if shouldRetry {
			log.WithError(err).Warningln("Retrying...")
		}

		return shouldRetry
	})
}

func WithBuildLog(retryable Retryable, log *common.BuildLogger) Retryable {
	return NewRetryableDecorator(retryable.Run, func(tries int, err error) bool {
		shouldRetry := retryable.ShouldRetry(tries, err)
		if shouldRetry {
			logger := log.WithFields(logrus.Fields{logrus.ErrorKey: err})
			logger.Warningln("Retrying...")
		}

		return shouldRetry
	})
}

type RetryableDecorator struct {
	run         func() error
	shouldRetry func(tries int, err error) bool
}

func NewRetryableDecorator(run func() error, shouldRetry func(tries int, err error) bool) *RetryableDecorator {
	return &RetryableDecorator{
		run:         run,
		shouldRetry: shouldRetry,
	}
}

func (d *RetryableDecorator) Run() error {
	return d.run()
}

func (d *RetryableDecorator) ShouldRetry(tries int, err error) bool {
	return d.shouldRetry(tries, err)
}
