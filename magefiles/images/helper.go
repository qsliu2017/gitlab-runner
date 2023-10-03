package images

import (
	"bytes"
	"fmt"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"text/template"
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
		Registry: ci.RegistryImage(),
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

type blueprintImpl []helperBuild

func (b blueprintImpl) Prerequisites() []string {
	return lo.Map(b, func(item helperBuild, _ int) string {
		return item.archive
	})
}

func (b blueprintImpl) Artifacts() []string {
	return lo.Flatten(lo.Map(b, func(item helperBuild, _ int) []string {
		return item.tags.all()
	}))
}

func (b blueprintImpl) Data() []helperBuild {
	return b
}

func AssembleReleaseHelper(flavor, prefix string) build.TargetBlueprint[string, string, []helperBuild] {
	var archs []string
	switch flavor {
	case "ubi-fips":
		archs = []string{"x86_64"}
	case "alpine-edge":
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le", "riscv64"}
	default:
		archs = []string{"x86_64", "arm", "arm64", "s390x", "ppc64le"}
	}

	var builds blueprintImpl
	for _, arch := range archs {
		builds = append(builds, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-%s.tar.xz", flavor, arch),
			platform: platformMap[arch],
			tags:     newHelperTagsList(prefix, "", arch),
		})
	}

	if lo.Contains(flavorsSupportingPWSH, flavor) {
		builds = append(builds, helperBuild{
			archive:  fmt.Sprintf("out/helper-images/prebuilt-%s-x86_64-pwsh.tar.xz", flavor),
			platform: platformMap["x86_64"],
			tags:     newHelperTagsList(prefix, "-pwsh", "arch"),
		})
	}

	return builds
}

func ReleaseHelper(blueprint build.TargetBlueprint[string, string, []helperBuild], publish bool) error {
	builder := docker.NewBuilder()

	logout, err := builder.LoginCI()
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
	archive := string(build.archive)
	baseTag := string(build.tags.revisionTag())
	latestTag := string(build.tags.latestTag())
	versionTag := string(build.tags.versionTag())

	if err := builder.Import(archive, baseTag, build.platform); err != nil {
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
