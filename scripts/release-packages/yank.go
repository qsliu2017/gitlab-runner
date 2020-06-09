package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	packageArchs = map[PackageType][]string{
		PackageDEB: {
			"amd64",
			"i386",
			"armel",
			"armhf",
			"arm64",
			"aarch64",
		},
		PackageRPM: {
			"x86_64",
			"i686",
			"arm",
			"armhf",
			"arm64",
			"aarch64",
		},
	}

	yankMethods = map[PackageType]func(wg *sync.WaitGroup, id string, version string, arch string){
		PackageDEB: yankDEB,
		PackageRPM: yankRPM,
	}

	failed = false
)

func yank() {
	if len(os.Args) < 3 || os.Args[2] == "" {
		panic("Missing version argument")
	}
	version := os.Args[2]

	wg := new(sync.WaitGroup)

	for _, distribution := range distributions.Distributions {
		for _, versionInfo := range distribution.Versions {
			for _, id := range versionInfo.IDs {
				for _, arch := range packageArchs[distribution.Type] {
					wg.Add(1)
					go yankMethods[distribution.Type](wg, id, version, arch)
				}
			}
		}
	}

	wg.Wait()

	if failed {
		logrus.Fatal("At least one of the yank operations failed")
	}
}

func yankDEB(wg *sync.WaitGroup, id string, version string, arch string) {
	defer wg.Done()

	yankPackage(id, fmt.Sprintf("%s_%s_%s.%s", packageName, version, arch, "deb"))
}

func yankPackage(id string, pkg string) {
	buf := new(bytes.Buffer)

	repo := fmt.Sprintf("%s/%s", packageCloudRepo, id)
	cmd := exec.Command("package_cloud")
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.Args = append(
		cmd.Args,
		"yank",
		"--url", packageCloudURL,
		repo,
		pkg,
	)

	err := cmd.Run()
	if err != nil {
		fmt.Println()
		logrus.WithError(err).Warningf("Failed to yank package %q from %s/%s", pkg, packageCloudURL, repo)
		fmt.Println(buf.String())

		failed = true

		return
	}

	logrus.Infof("Yanked package %q from %s/%s", pkg, packageCloudURL, repo)
}

func yankRPM(wg *sync.WaitGroup, id string, version string, arch string) {
	defer wg.Done()

	yankPackage(id, fmt.Sprintf("%s-%s-1.%s.%s", packageName, version, arch, "deb"))
}
