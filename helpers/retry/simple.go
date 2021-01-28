package retry

type Simple struct {
	retryable Retryable
}

func NewSimple(retryable Retryable) *Simple {
	return &Simple{
		retryable: retryable,
	}
}

func (r *Simple) Run() error {
	return r.loop(nil)
}

func (r *Simple) loop(fn func()) error {
	var err error
	var tries int

	for {
		tries++
		err = r.retryable.Run()
		if err == nil || !r.retryable.ShouldRetry(tries, err) {
			break
		}

		if fn != nil {
			fn()
		}
	}

	if err != nil {
		return newErrRetriesExceeded(tries, err)
	}

	return nil
}
