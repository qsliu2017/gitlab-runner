//go:build mage

package main

import (
	"bytes"
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/images"
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

func (Images) ReleaseHelper(flavor, prefix string) error {
	return releaseHelper(flavor, prefix)
}

type helperBuild struct {
	archive  string
	platform string
	tags     helperTagsList
}

type helperTagsList struct {
	suffix       string
	baseTemplate string
	prefix       string
	arch         string
}

func newHelperTagsList(prefix, suffix, arch string) helperTagsList {
	return helperTagsList{
		prefix:       prefix,
		suffix:       suffix,
		arch:         arch,
		baseTemplate: "{{ .Registry }}/gitlab-runner-helper:{{ .Prefix }}{{ .Arch}}-"}
}

func (l helperTagsList) render(raw string) string {
	context := struct {
		Registry string
		Prefix   string
		Arch     string
		Revision string
		RefTag   string
	}{
		Registry: ci.RegistryImage,
		Prefix:   l.prefix,
		Arch:     l.arch,
		Revision: build.Revision(),
		RefTag:   build.RefTag(),
	}

	var out bytes.Buffer
	tmpl := lo.Must(template.New("tmpl").Parse(l.baseTemplate + raw + l.suffix))

	lo.Must0(tmpl.Execute(&out, &context))

	return out.String()
}

func (l helperTagsList) revisionTag() string {
	return l.render("{{ .Revision }}")
}

func (l helperTagsList) versionTag() string {
	return l.render("{{ .RefTag }}")
}

func (l helperTagsList) latestTag() string {
	return l.render("latest")
}

func (l helperTagsList) all() []string {
	return []string{
		l.revisionTag(),
		l.versionTag(),
		l.latestTag(),
	}
}

func releaseHelper(flavor, prefix string) error {
	var archs []string
	switch flavor {
	case "ubi-fips":
		archs = []string{"x86_64"}
	case "alpine-edge":
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le", "riscv64"}
	default:
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le"}
	}

	var builds []helperBuild
	for _, arch := range archs {
		builds = append(builds, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-%s.tar.xz", flavor, arch),
			platform: platformMap[arch],
			tags:     newHelperTagsList(prefix, "", arch),
		})
	}

	if flavor != "alpine-edge" && flavor != "alpine-latest" {
		builds = append(builds, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-x86_64-pwsh.tar.xz", flavor),
			platform: platformMap["x86_64"],
			tags:     newHelperTagsList(prefix, "-pwsh", "arch"),
		})
	}

	fmt.Println("========")
	fmt.Println("This target requires:")
	fmt.Println(strings.Join(lo.Map(builds, func(build helperBuild, _ int) string {
		return build.archive
	}), "\n"))
	fmt.Println("This target produces:")
	fmt.Println(strings.Join(lo.Map(builds, func(build helperBuild, _ int) string {
		return strings.Join(build.tags.all(), "\n")
	}), "\n"))
	fmt.Println("========")

	builder := docker.NewBuilder()

	logout, err := builder.LoginCI()
	if err != nil {
		return err
	}
	defer logout()

	for _, build := range builds {
		if err := releaseImage(
			builder,
			build,
			false, // TODO:
		); err != nil {
			return err
		}
	}

	return nil
}

func releaseImage(builder *docker.Builder, build helperBuild, publish bool) error {
	baseTag := build.tags.revisionTag()
	latestTag := build.tags.latestTag()
	versionTag := build.tags.versionTag()

	if err := builder.Import(build.archive, baseTag, build.platform); err != nil {
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
