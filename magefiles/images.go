//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/images"
)

type Images mg.Namespace

func (Images) BuildRunnerDefault() error {
	return runRunnerBuild(images.DefaultFlavor, images.DefaultArchs, false)
}

func (Images) BuildRunner(flavor, targetArchs string) error {
	return runRunnerBuild(flavor, targetArchs, false)
}

func (Images) ReleaseRunner(flavor, targetArchs string) error {
	return runRunnerBuild(flavor, targetArchs, true)
}

func runRunnerBuild(flavor, targetArchs string, publish bool) error {
	blueprint := build.PrintBlueprint(images.AssembleBuildRunner(flavor, targetArchs))
	if err := build.Export(blueprint.Artifacts(), build.ReleaseArtifactsPath("runner_images")); err != nil {
		return err
	}

	return images.BuildRunner(blueprint, publish)
}

func (Images) TagHelperDefault() error {
	return runHelperBuild(images.DefaultFlavor, "", false)
}

func (Images) TagHelper(flavor, prefix string) error {
	return runHelperBuild(flavor, prefix, false)
}

func (Images) ReleaseHelper(flavor, prefix string) error {
	return runHelperBuild(flavor, prefix, true)
}

func runHelperBuild(flavor, prefix string, publish bool) error {
	blueprint := build.PrintBlueprint(images.AssembleReleaseHelper(flavor, prefix))
	if err := build.Export(blueprint.Artifacts(), build.ReleaseArtifactsPath("helper_images")); err != nil {
		return err
	}

	return images.ReleaseHelper(blueprint, publish)
}
