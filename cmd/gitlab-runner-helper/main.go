package main

import (
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/app"
	"gitlab.com/gitlab-org/gitlab-runner/log"

	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
)

func main() {
	defer app.Recover()

	a := app.New("GitLab Runner Helper")
	log.AddSecretsCleanupLogHook(logrus.StandardLogger())

	a.Run()
}
