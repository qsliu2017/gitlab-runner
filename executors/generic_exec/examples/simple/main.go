package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
)

const (
	StagePrepare = "prepare"
	StageRun     = "run"
	StageCleanup = "cleanup"
)

const (
	JobFailureExitCodeEnv    = "GENERIC_BUILD_FAILURE_EXIT_CODE"
	SystemFailureExitCodeEnv = "GENERIC_SYSTEM_FAILURE_EXIT_CODE"
)

type callback func(ctx context.Context, args []string) error

func main() {
	stage := os.Args[1]
	args := os.Args[2:]

	ctx, ctxCancelFn := setupSignals()
	defer ctxCancelFn()

	callbacks := map[string]callback{
		StagePrepare: prepare,
		StageRun:     run,
		StageCleanup: cleanup,
	}

	callbackFn, ok := callbacks[stage]
	if !ok {
		systemFail("Unsupported stage %q", stage)
	}

	err := callbackFn(ctx, args)
	if err != nil {
		systemFail("Error during stage handling: %v", err)
	}
}

func setupSignals() (context.Context, func()) {
	ctx, cancelFn := context.WithCancel(context.Background())

	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case signal := <-signalCh:
			fmt.Printf("Received signal %v\n", signal)
			cancelFn()
		}
	}()

	terminate := func() {
		close(signalCh)
	}

	return ctx, terminate
}

func prepare(ctx context.Context, args []string) error {
	fmt.Println("Not implemented; skipping")

	return nil
}

func run(ctx context.Context, args []string) error {
	fmt.Printf("Executing %q stage\n", args[1])

	cmd := exec.CommandContext(ctx, "bash", args[0])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("couldn't start the job script execution: %v", err)
	}

	pid := cmd.Process.Pid
	waitCh := make(chan error)

	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		err = syscall.Kill(-pid, syscall.SIGTERM)
		if err == nil {
			return nil
		}

		return fmt.Errorf("job script couldn't be terminated: %v", err)

	case err = <-waitCh:
		if err == nil {
			return nil
		}

		if eerr, ok := err.(*exec.ExitError); ok {
			// TODO: simplify when we will update to Go 1.12. ExitStatus()
			//       is available there directly from err.Sys().
			exitCode := eerr.Sys().(syscall.WaitStatus).ExitStatus()

			jobFail("Job script execution have failed with exit code: %d", exitCode)
		}

		return fmt.Errorf("job script execution terminated with unexpected error: %v", err)
	}
}

func cleanup(ctx context.Context, args []string) error {
	fmt.Println("Not implemented; skipping")

	return nil
}

func jobFail(msg string, args ...interface{}) {
	fail(JobFailureExitCodeEnv, msg, args...)
}

func systemFail(msg string, args ...interface{}) {
	fail(SystemFailureExitCodeEnv, msg, args...)
}

func fail(exitCodeVariable string, msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	fmt.Println()

	codeString := os.Getenv(exitCodeVariable)

	exitCode, err := strconv.Atoi(codeString)
	if err != nil {
		panic(fmt.Sprintf("Couldn't parse exit code variable %q value: %s", exitCodeVariable, codeString))
	}

	os.Exit(exitCode)
}
