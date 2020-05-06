package helpers

import (
	"os/exec"
	"sync"
	"testing"
)

var (
	integrationExecutionSkipCache sync.Once
	integrationPowershellSkip     = true
	integrationBashSkip           = true
	integrationDockerSkip         = true
	integrationKubectlSkip        = true
	integrationPrlCtlSkip         = true
	integrationVBboxManageSkip    = true
)

func skipIntegrationExecutionCache() {
	integrationExecutionSkipCache.Do(func() {
		integrationPowershellSkip = exec.Command("powershell").Run() != nil
		integrationBashSkip = exec.Command("bash").Run() != nil
		integrationDockerSkip = exec.Command("docker", "info").Run() != nil
		integrationKubectlSkip = exec.Command("kubectl", "cluster-info").Run() != nil
		integrationPrlCtlSkip = exec.Command("prlctl", "--version").Run() != nil
		integrationVBboxManageSkip = exec.Command("vboxmanage", "--version").Run() != nil
	})
}

// SkipDockerIntegrationTests skips docker tests if `docker info` fails to
// execute.
func SkipDockerIntegrationTests(t *testing.T) {
	skipIntegrationExecutionCache()
	if integrationDockerSkip {
		t.Skip("skipping docker integration tests")
	}
}

// SkipKubernetesIntegrationTests skips kubernetes tests if
// `kubectl cluster-info` fails to execute.
func SkipKubernetesIntegrationTests(t *testing.T) {
	skipIntegrationExecutionCache()
	if integrationKubectlSkip {
		t.Skip("skipping kubernetes integration tests")
	}
}

// SkipParallelsIntegrationTests skips parallels tests if `prlctl --version`
// fails to execute.
func SkipParallelsIntegrationTests(t *testing.T) {
	skipIntegrationExecutionCache()
	if integrationPrlCtlSkip {
		t.Skip("skipping parallels integration tests")
	}
}

// SkipVirtualBoxIntegrationTests skips virtualbox tests if
// `vboxmanage --version` fails to execute.
func SkipVirtualBoxIntegrationTests(t *testing.T) {
	skipIntegrationExecutionCache()
	if integrationVBboxManageSkip {
		t.Skip("skipping virtualbox integration tests")
	}
}

// SkipShellIntegrationTests skips shell tests for the shell name specified if
// the shell command fails to execute.
func SkipShellIntegrationTests(t *testing.T, shell string) {
	skipIntegrationExecutionCache()
	switch shell {
	case "powershell":
		if integrationPowershellSkip {
			t.Skip("skipping powershell integration tests")
		}

	case "bash":
		if integrationBashSkip {
			t.Skip("skipping bash integration tests")
		}

	default:
		t.Errorf("please add skipShellIntegrationTest for %q", shell)
	}
}
