//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/images"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

type Images mg.Namespace

func (i Images) ReleaseRunnerDefault() error {
	return i.BuildRunner(images.DefaultFlavor, images.DefaultArchs)
}

func (Images) BuildRunner(flavor, targetArchs string) error {
	defer mageutils.PrintUsedVariables(config.Verbose)

	return images.BuildRunner(flavor, targetArchs, false)
}

func (Images) ReleaseRunner(flavor, targetArchs string) error {
	defer mageutils.PrintUsedVariables(config.Verbose)

	return images.BuildRunner(flavor, targetArchs, true)
}

func (Images) TagHelper(flavor, prefix string) error {
	defer mageutils.PrintUsedVariables(config.Verbose)

	blueprint := images.AssembleReleaseHelper(flavor, prefix)
	build.PrintBlueprint(blueprint)

	return images.ReleaseHelper(blueprint, false)
}

func (Images) ReleaseHelper(flavor, prefix string) error {
	defer mageutils.PrintUsedVariables(config.Verbose)

	blueprint := images.AssembleReleaseHelper(flavor, prefix)
	build.PrintBlueprint(blueprint)

	return images.ReleaseHelper(blueprint, true)
}
