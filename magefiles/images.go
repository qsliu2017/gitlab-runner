//go:build mage

package main

import (
	"bytes"
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/images"
	"os"
	"strings"
	"text/template"
)

type Images mg.Namespace

func (i Images) ReleaseRunnerDefault() error {
	return i.BuildRunner(images.DefaultFlavor, images.DefaultArchs)
}

func (Images) BuildRunner(flavor, targetArchs string) error {
	return images.BuildRunner(flavor, targetArchs, false)
}

func (Images) ReleaseRunner(flavor, targetArchs string) error {
	return images.BuildRunner(flavor, targetArchs, true)
}

var platformMap = map[string]string{
	"x86_64":  "linux/amd64",
	"arm":     "linux/arm/v7",
	"arm64":   "linux/arm64/v8",
	"s390x":   "linux/s390x",
	"ppc64le": "linux/ppc64le",
	"riscv64": "linux/riscv64",
}

func (Images) ReleaseHelper(flavor, tag string) error {
	builder := docker.NewBuilder()

	logout, err := builder.LoginCI()
	if err != nil {
		return err
	}
	defer logout()

	baseTemplate := "{{ .Registry }}/gitlab-runner-helper:{{ .Tag }}{{ .Arch}}-"
	baseVariantsTemplates := []string{
		fmt.Sprintf("%s{{ .Revision }}", baseTemplate),
		fmt.Sprintf("%s{{ .RefTag }}", baseTemplate),
		fmt.Sprintf("%slatest", baseTemplate),
		fmt.Sprintf("%slatest", baseTemplate),
	}

	archs := []string{"x86_64", "arm", "arm64", "s390x", "ppc64le"}

	switch flavor {
	case "ubi-fips":
		archs = []string{"x86_64"}
	case "alpine-edge":
		archs = append(archs, "riscv64")
	}

	for _, arch := range archs {
		tags, err := generateVariants(arch, tag, build.RefTag(), baseVariantsTemplates)
		if err != nil {
			return err
		}

		archive := fmt.Sprintf("out/helper-images/prebuilt-%s-%s.tar.xz", flavor, arch)
		if err := releaseImage(
			builder,
			tags,
			archive,
			arch,
			"-latest",
			fmt.Sprintf("-%s", build.RefTag()),
			true,
		); err != nil {
			return err
		}
	}

	if flavor != "alpine-edge" && flavor != "alpine-latest" {
		tags, err := generateVariants("x86_64", tag, build.RefTag(), []string{
			"{{ .Revision}}-pwsh",
			"{{ .RefTag }}-pwsh",
			"latest-pwsh",
		})
		if err != nil {
			return err
		}

		archive := fmt.Sprintf("out/helper-images/prebuilt-%s-x86_64-pwsh.tar.xz", flavor)
		if err := releaseImage(
			builder,
			tags,
			archive,
			"x86_64",
			"-latest-pwsh",
			fmt.Sprintf("-%s-pwsh", build.RefTag()),
			true,
		); err != nil {
			return err
		}
	}

	return nil
}

func releaseImage(builder *docker.Builder, tags []string, archive, arch, latestSuffix, versionSuffix string, publish bool) error {
	baseTag := tags[0]
	latestTag, _ := lo.Find(tags, func(tag string) bool {
		return strings.HasSuffix(tag, latestSuffix)
	})

	versionTag, _ := lo.Find(tags, func(tag string) bool {
		return strings.HasSuffix(tag, versionSuffix)
	})

	if err := releaseDockerImages(
		builder,
		archive,
		baseTag,
		latestTag,
		versionTag,
		arch,
		publish,
	); err != nil {
		return err
	}

	return nil
}

func releaseDockerImages(builder *docker.Builder, archive, baseTag, latestTag, versionTag, arch string, publish bool) error {
	if err := builder.Import(archive, baseTag, platformMap[arch]); err != nil {
		return err
	}

	if err := builder.TagLatest(baseTag, latestTag); err != nil {
		return err
	}

	if err := builder.Tag(baseTag, versionTag); err != nil {
		return err
	}

	if !publish {
		return nil
	}

	for _, tag := range []string{baseTag, latestTag, versionTag} {
		if err := builder.Push(tag); err != nil {
			return err
		}
	}

	return nil
}

func generateVariants(arch, tag, refTag string, templates []string) ([]string, error) {
	registry := os.Getenv("CI_REGISTRY_IMAGE")

	context := struct {
		Registry string
		Tag      string
		Arch     string
		Revision string
		RefTag   string
	}{
		Registry: registry,
		Tag:      tag,
		Arch:     arch,
		Revision: build.Revision(),
		RefTag:   refTag,
	}

	var variants []string

	for _, t := range templates {
		var out bytes.Buffer
		tmpl, err := template.New(arch).Parse(t)
		if err != nil {
			return nil, err
		}

		if err := tmpl.Execute(&out, &context); err != nil {
			return nil, err
		}

		variants = append(variants, out.String())
	}

	return variants, nil
}
