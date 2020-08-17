package common

import (
	"io"
	"os"
	"sync"
)

type Trace struct {
	Writer     io.Writer
	cancelFunc CancelFunc
	mutex      sync.Mutex
}

func (s *Trace) Write(p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.Writer == nil {
		return 0, os.ErrInvalid
	}
	return s.Writer.Write(p)
}

func (s *Trace) SetMasked(values []string) {
}

func (s *Trace) Success() {
}

func (s *Trace) Fail(err error, failureReason JobFailureReason) {
}

func (s *Trace) SetCancelFunc(cancelFunc CancelFunc) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cancelFunc = cancelFunc
}

func (s *Trace) Cancel(remoteJobState JobState) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cancelFunc == nil {
		return false
	}

	s.cancelFunc(remoteJobState)
	return true
}

func (s *Trace) SetFailuresCollector(fc FailuresCollector) {}

func (s *Trace) IsStdout() bool {
	return true
}
