package retry

import (
	"fmt"
)

type ErrRetriesExceeded struct {
	tries int
	inner error
}

func newErrRetriesExceeded(tries int, inner error) *ErrRetriesExceeded {
	return &ErrRetriesExceeded{
		tries: tries,
		inner: inner,
	}
}

func (e *ErrRetriesExceeded) Error() string {
	return fmt.Sprintf("limit of %d retries exceeded: %v", e.tries, e.inner)
}

func (e *ErrRetriesExceeded) Unwrap() error {
	return e.inner
}
