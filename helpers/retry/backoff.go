package retry

import (
	"time"

	"github.com/jpillora/backoff"
)

const (
	defaultRetryMinBackoff = 1 * time.Second
	defaultRetryMaxBackoff = 5 * time.Second
)

type Backoff struct {
	inner   *Simple
	backoff *backoff.Backoff
}

func NewBackoff(retryable Retryable) *Backoff {
	return &Backoff{
		inner:   NewSimple(retryable),
		backoff: &backoff.Backoff{Min: defaultRetryMinBackoff, Max: defaultRetryMaxBackoff},
	}
}

func (r *Backoff) Run() error {
	return r.inner.loop(func() {
		time.Sleep(r.backoff.Duration())
	})
}
