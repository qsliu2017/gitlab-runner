package images

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
)

var platformMap = map[string]string{
	"x86_64":  "linux/amd64",
	"arm":     "linux/arm/v7",
	"arm64":   "linux/arm64/v8",
	"s390x":   "linux/s390x",
	"ppc64le": "linux/ppc64le",
	"riscv64": "linux/riscv64",
}

var flavorsSupportingPWSH = []string{
	"alpine",
	"alpine3.15",
	"alpine3.16",
	"alpine3.17",
	"alpine3.18",
	"ubuntu",
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
	registry     string
}

func newHelperTagsList(prefix, suffix, arch, registry string) helperTagsList {
	return helperTagsList{
		prefix:       prefix,
		suffix:       suffix,
		arch:         arch,
		registry:     registry,
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
		Registry: l.registry,
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

type helperBlueprintImpl struct {
	build.BlueprintBase

	data []helperBuild
}

func (b helperBlueprintImpl) Dependencies() []build.Component {
	return lo.Map(b.data, func(item helperBuild, _ int) build.Component {
		return build.NewDockerImageArchive(item.archive)
	})
}

func (b helperBlueprintImpl) Artifacts() []build.Component {
	return lo.Flatten(lo.Map(b.data, func(item helperBuild, _ int) []build.Component {
		return lo.Map(item.tags.all(), func(item string, _ int) build.Component {
			return build.NewDockerImage(item)
		})
	}))
}

func (b helperBlueprintImpl) Data() []helperBuild {
	return b.data
}

func AssembleReleaseHelper(flavor, prefix string) build.TargetBlueprint[build.Component, build.Component, []helperBuild] {
	var archs []string
	switch flavor {
	case "ubi-fips":
		archs = []string{"x86_64"}
	case "alpine-edge":
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le", "riscv64"}
	default:
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le"}
	}

	builds := helperBlueprintImpl{
		BlueprintBase: build.NewBlueprintBase(ci.RegistryAuthBundle, docker.BuilderEnvBundle),
		data:          []helperBuild{},
	}

	registry := builds.Env().Value(ci.Registry)

	for _, arch := range archs {
		builds.data = append(builds.data, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-%s.tar.xz", flavor, arch),
			platform: platformMap[arch],
			tags:     newHelperTagsList(prefix, "", arch, registry),
		})
	}

	if lo.Contains(flavorsSupportingPWSH, flavor) {
		builds.data = append(builds.data, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-x86_64-pwsh.tar.xz", flavor),
			platform: platformMap["x86_64"],
			tags:     newHelperTagsList(prefix, "-pwsh", "arch", registry),
		})
	}

	return builds
}

func ReleaseHelper(blueprint build.TargetBlueprint[build.Component, build.Component, []helperBuild], publish bool) error {
	builder := docker.NewBuilder(
		blueprint.Env().Value(docker.Host),
		blueprint.Env().Value(docker.CertPath),
	)

	logout, err := builder.Login(
		blueprint.Env().Value(ci.RegistryUser),
		blueprint.Env().Value(ci.RegistryPassword),
		blueprint.Env().Value(ci.Registry),
	)
	if err != nil {
		return err
	}
	defer logout()

	for _, build := range blueprint.Data() {
		if err := releaseImage(
			builder,
			build,
			publish,
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
