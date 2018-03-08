package network

// import (
// 	"context"
// 	"io"
// )

// type JobFailureReason string

// const (
// 	NoneFailure         JobFailureReason = ""
// 	ScriptFailure       JobFailureReason = "script_failure"
// 	RunnerSystemFailure JobFailureReason = "runner_system_failure"
// )

// type JobTrace interface {
// 	io.Writer
// 	Success()
// 	Fail(err error, failureReason JobFailureReason)
// 	SetCancelFunc(cancelFunc context.CancelFunc)
// 	SetFailuresCollector(fc FailuresCollector)
// 	IsStdout() bool
// }

// type FailuresCollector interface {
// 	RecordFailure(reason JobFailureReason, runnerDescription string)
// }
