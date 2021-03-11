package shells

import (
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/shells/runnershell"
)

func WrapShell(shell common.Shell) common.Shell {
	return &RunnerShell{Shell: shell}
}

type RunnerShell struct {
	Shell common.Shell
}

func (s *RunnerShell) GetName() string {
	return s.Shell.GetName()
}

func (s *RunnerShell) GetFeatures(features *common.FeaturesInfo) {
	s.Shell.GetFeatures(features)
}

func (s *RunnerShell) IsDefault() bool {
	return s.Shell.IsDefault()
}

func (s *RunnerShell) GetConfiguration(info common.ShellScriptInfo) (*common.ShellConfiguration, error) {
	return s.Shell.GetConfiguration(info)
}

func (s *RunnerShell) GenerateScript(buildStage common.BuildStage, info common.ShellScriptInfo) (string, error) {
	if !info.Build.IsFeatureFlagOn(featureflags.UseRunnerShell) {
		return s.Shell.GenerateScript(buildStage, info)
	}

	if !common.GetPredefinedEnv(buildStage) {
		return s.Shell.GenerateScript(buildStage, info)
	}

	writer := &runnershell.Writer{}
	writer.Script.TemporaryPath = info.Build.TmpProjectDir()

	cfg, err := s.Shell.GetConfiguration(info)
	if err != nil {
		return "", err
	}

	// Most arbitrary user scripts go to the regular shell. For predefined
	// build stages there's still a couple of user scripts that can be run.
	//
	// For handling these, we open whatever command the underlying shell
	// would typically use and write to stdin any script provided.
	if !cfg.PassFile {
		writer.Script.ShellCommand = append([]string{cfg.Command}, cfg.Arguments...)
	}

	as := new(AbstractShell)
	if err := as.writeScript(writer, buildStage, info); err != nil {
		return "", err
	}

	return writer.Finish(info.Build.IsDebugTraceEnabled()), nil
}
