package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/executors/custom/api"
)

const (
	isBuildError   = "CUSTOM_ENV_IS_BUILD_ERROR"
	isSystemError  = "CUSTOM_ENV_IS_SYSTEM_ERROR"
	isUnknownError = "CUSTOM_ENV_IS_UNKNOWN_ERROR"
)

const (
	stageConfig  = "config"
	stagePrepare = "prepare"
	stageRun     = "run"
	stageCleanup = "cleanup"
)

func validateBuildStageName(stage string) {
	const stepStagePrefix = "step_"

	var knownBuildStages = map[string]struct{}{
		"prepare_script":              {},
		"get_sources":                 {},
		"restore_cache":               {},
		"download_artifacts":          {},
		"after_script":                {},
		"archive_cache":               {},
		"archive_cache_on_failure":    {},
		"upload_artifacts_on_success": {},
		"upload_artifacts_on_failure": {},
	}

	if _, ok := knownBuildStages[stage]; ok {
		return
	}

	if strings.HasPrefix(stage, stepStagePrefix) {
		return
	}

	// TODO: Remove this in 15.0 - https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27959
	if stage == "build_script" {
		return
	}

	setSystemFailure("Unknown build stage %q", stage)
}

func setBuildFailure(msg string, args ...interface{}) {
	fmt.Println("setting build failure")
	setFailure(api.BuildFailureExitCodeVariable, msg, args...)
}

func setSystemFailure(msg string, args ...interface{}) {
	fmt.Println("setting system failure")
	setFailure(api.SystemFailureExitCodeVariable, msg, args...)
}

func setFailure(failureType string, msg string, args ...interface{}) {
	fmt.Println()
	fmt.Printf(msg, args...)
	fmt.Println()

	exitCode := os.Getenv(failureType)

	code, err := strconv.Atoi(exitCode)
	if err != nil {
		panic(fmt.Sprintf("Error while parsing the variable: %v", err))
	}

	fmt.Printf("Exitting with code %d\n", code)

	os.Exit(code)
}

type stageFunc func(shell string, args []string)

func main() {
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		setSystemFailure("Executor panicked with: %v", r)
	}()

	shell := os.Args[1]
	stage := os.Args[2]

	var args []string
	if len(os.Args) > 3 {
		args = os.Args[3:]
	}

	stages := map[string]stageFunc{
		stageConfig:  config,
		stagePrepare: prepare,
		stageRun:     run,
		stageCleanup: cleanup,
	}

	stageFn, ok := stages[stage]
	if !ok {
		setSystemFailure("Unknown stage %q", stage)
	}

	_, _ = fmt.Fprintf(os.Stderr, "Custom Executor binary - %q stage\n", stage)
	_, _ = fmt.Fprintf(os.Stderr, "Mocking execution of: %v\n", args)
	_, _ = fmt.Fprintln(os.Stderr)

	stageFn(shell, args)
}

func config(shell string, args []string) {
	var config api.ConfigExecOutput

	adjustConfig(&config, args)

	jsonOutput, err := json.Marshal(config)
	if err != nil {
		panic(fmt.Errorf("error while creating JSON output: %w", err))
	}

	fmt.Print(string(jsonOutput))
}

func adjustConfig(config *api.ConfigExecOutput, args []string) {
	for _, arg := range args {
		setBuildsDirConfig(config, arg)
		setStepScriptSupportedConfig(config, arg)
	}
}

func setBuildsDirConfig(config *api.ConfigExecOutput, arg string) {
	const (
		argumentPrefix = "builds_dir="
	)

	if !strings.HasPrefix(arg, argumentPrefix) {
		return
	}

	dir := strings.TrimPrefix(arg, argumentPrefix)

	config.BuildsDir = &dir
}

// setStepScriptSupportedConfig
// DEPRECATED
// TODO: Remove this in 15.0 - https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27959
func setStepScriptSupportedConfig(config *api.ConfigExecOutput, arg string) {
	const (
		argumentPrefix = "step_script_supported="
	)

	if !strings.HasPrefix(arg, argumentPrefix) {
		return
	}

	stepScriptSupported, err := strconv.ParseBool(strings.TrimPrefix(arg, argumentPrefix))
	if err != nil {
		panic(fmt.Sprintf(
			"invalid value of %q argument; expected bool got %v",
			argumentPrefix,
			stepScriptSupported,
		))
	}

	config.StepScriptSupported = &stepScriptSupported
}

func prepare(shell string, args []string) {
	fmt.Println("PREPARE doesn't accept any arguments. It just does its job")
	fmt.Println()
}

func run(shell string, args []string) {
	fmt.Println("RUN accepts two arguments: the path to the script to execute and the stage of the job")
	fmt.Println()

	mockError()

	if len(args) < 1 {
		setSystemFailure("Missing script for the run stage")
	}

	output := bytes.NewBuffer(nil)

	cmd := createCommand(shell, args[0], args[1])
	cmd.Stdout = output
	cmd.Stderr = output

	fmt.Printf("Executing: %#v\n\n", cmd)

	err := cmd.Run()
	if err != nil {
		setBuildFailure("Job script exited with: %v", err)
	}

	fmt.Printf(">>>>>>>>>>\n%s\n<<<<<<<<<<\n\n", output.String())
}

func mockError() {
	if len(os.Getenv(isBuildError)) > 0 {
		// It's a build error. For example: user used an invalid
		// command in his script which ends with an error thrown
		// from the underlying shell.

		setBuildFailure("mocked build failure")
	}

	if len(os.Getenv(isSystemError)) > 0 {
		// It's a system error. For example: the Custom Executor
		// script implements a libvirt executor and before executing
		// the job it needs to prepare the VM. But the preparation
		// failed.

		setSystemFailure("mocked system failure")
	}

	if len(os.Getenv(isUnknownError)) > 0 {
		// This situation should not happen. Custom Executor script
		// should define the type of failure and return either "build
		// failure" or "system failure", using the error code values
		// provided by dedicated variables.

		fmt.Println("mocked system failure")
		os.Exit(255)
	}
}

func createCommand(shell string, script string, stage string) *exec.Cmd {
	validateBuildStageName(stage)

	shellConfigs := map[string]struct {
		command string
		args    []string
	}{
		"bash": {
			command: "bash",
			args:    []string{},
		},
		"powershell": {
			command: "powershell",
			args:    []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command"},
		},
		"pwsh": {
			command: "pwsh",
			args:    []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command"},
		},
		"cmd": {
			command: "cmd",
			args:    []string{"/C"},
		},
	}

	shellConfig, ok := shellConfigs[shell]
	if !ok {
		panic(fmt.Sprintf("Unknown shell %q", shell))
	}

	args := append(shellConfig.args, script)

	return exec.Command(shellConfig.command, args...)
}

func cleanup(shell string, args []string) {
	fmt.Println("CLEANUP doesn't accept any arguments. It just does its job")
	fmt.Println()
}
