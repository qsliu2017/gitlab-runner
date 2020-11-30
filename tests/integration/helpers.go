package testcli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func registerRunner(flags string) error {
	return runRunner("register " + flags)
}

func runRunner(flags string) error {
	cmd := exec.Command("gitlab-runner", strings.Split(flags, " ")...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println("Runner Output: " + string(stdout))
		fmt.Println("Runner Error: " + stderr.String())
	}

	return err
}

func diffConfigs(expected string) error {
	cmd := exec.Command("diff", "-c", TestConfig, expected)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()

	if err != nil {
		fmt.Println("Output: " + string(stdout))
		fmt.Println("Error: " + stderr.String())
	}

	return err
}

func cleanUp() {
	os.Remove(TestConfig)
}

func expectedFileNameFromTestname(testname string) string {
	testname = strings.ToLower(strings.TrimPrefix(testname, "Test"))
	return fmt.Sprintf("expected-outputs/%s.toml", strings.ReplaceAll(testname, " ", "-"))
}
