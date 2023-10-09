package main

import (
	"github.com/magefile/mage/mg"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/images"
)

type Images mg.Namespace

func (i Images) BuildRunnerDefault() error {
	blueprint := build.PrintBlueprint(images.AssembleBuildRunner(images.DefaultFlavor, images.DefaultArchs))
	return images.BuildRunner(blueprint, false)
}

func (Images) BuildRunner(flavor, targetArchs string) error {
	blueprint := build.PrintBlueprint(images.AssembleBuildRunner(flavor, targetArchs))
	return images.BuildRunner(blueprint, false)
}

func (Images) ReleaseRunner(flavor, targetArchs string) error {
	blueprint := build.PrintBlueprint(images.AssembleBuildRunner(flavor, targetArchs))
	return images.BuildRunner(blueprint, true)
}

func (Images) TagHelperDefault() error {
	blueprint := build.PrintBlueprint(images.AssembleReleaseHelper(images.DefaultFlavor, ""))
	return images.ReleaseHelper(blueprint, false)
}

func (Images) TagHelper(flavor, prefix string) error {
	blueprint := build.PrintBlueprint(images.AssembleReleaseHelper(flavor, prefix))
	return images.ReleaseHelper(blueprint, false)
}

func (Images) ReleaseHelper(flavor, prefix string) error {
	blueprint := build.PrintBlueprint(images.AssembleReleaseHelper(flavor, prefix))
	return images.ReleaseHelper(blueprint, true)
}
