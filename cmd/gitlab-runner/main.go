package main

import (
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/app"

	_ "gitlab.com/gitlab-org/gitlab-runner/cache/gcs"
	_ "gitlab.com/gitlab-org/gitlab-runner/cache/s3"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker/machine"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/parallels"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/shell"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/ssh"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/virtualbox"
	_ "gitlab.com/gitlab-org/gitlab-runner/shells"
)

func main() {
	defer app.Recover()

	a := app.New("GitLab Runner")
	a.AppendBeforeFunc(app.LogRuntimePlatform)
	a.Extend(app.CPUProfileFlags)
	a.AppendBeforeFunc(app.CPUProfileSetup)
	a.AppendAfterFunc(app.CPUProfileTeardown)
	a.AppendBeforeFunc(app.FixHOME)
	app.WarnOnBool(os.Args)

	a.Run()
}
