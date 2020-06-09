package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

func release() {
	target := "stable"
	unstable := false
	repo := packageCloudRepo

	targetID := strings.Split(os.Args[2], " ")
	if len(targetID) > 1 {
		target = targetID[0]
	}

	switch target {
	case "bleeding":
		unstable = true
		repo = packageCloudRepoBeta
	case "stable":
	default:
		panic(fmt.Sprintf("Unknown package upload target %q", target))
	}

	for _, distribution := range distributions.Distributions {
		for _, versionInfo := range distribution.Versions {
			for _, id := range versionInfo.IDs {
				if unstable && !versionInfo.PublishToUnstable {
					continue
				}

				tryUpload(id, distribution.Type, repo)
			}
		}
	}
}

func tryUpload(id string, packageType PackageType, repo string) {
	buf := new(bytes.Buffer)
	repo = fmt.Sprintf("%s/%s", repo, id)

	cmd := exec.Command("package_cloud")
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Args = append(
		cmd.Args,
		"push",
		"--url", packageCloudURL,
		repo,
		fmt.Sprintf("out/%s/*.%s", packageType, packageType),
	)

	err := cmd.Run()
	if err != nil {
		fmt.Println()
		logrus.WithError(err).Warningf("Failed to push packages to %s/%s", packageCloudURL, repo)
		fmt.Println(buf.String())

		return
	}

	logrus.WithError(err).Warningf("Pushed packages to %s/%s", packageCloudURL, repo)
}
