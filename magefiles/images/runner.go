package images

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/sh"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
)

const (
	DefaultFlavor = "ubuntu"
	DefaultArchs  = "amd64"

	runnerHomeDir = "dockerfiles/runner"
)

var (
	runnerImageName = env.New("RUNNER_IMAGE_NAME")

	dockerMachineVersion       = env.NewDefault("DOCKER_MACHINE_VERSION", "v0.16.2-gitlab.21")
	dockerMachineAmd64Checksum = env.NewDefault("DOCKER_MACHINE_AMD64_CHECKSUM", "a4e9a416f30406772e76c3b9e795121d5a7e677978923f96b7fb72f0d8354740")
	dockerMachineArm64Checksum = env.NewDefault("DOCKER_MACHINE_ARM64_CHECKSUM", "124ceefbe1a1eec44eeb932edf9f85dab1e532d449f5e3e236faed5e8b19caba")
	// s390x and ppc64le are not being released
	dockerMachineS390xChecksum   = env.New("DOCKER_MACHINE_S390X_CHECKSUM")
	dockerMachinePpc64leChecksum = env.New("DOCKER_MACHINE_PPC64LE_CHECKSUM")

	dumbInitVersion         = env.NewDefault("DUMB_INIT_VERSION", "1.2.2")
	dumbInitAmd64Checksum   = env.NewDefault("DUMB_INIT_AMD64_CHECKSUM", "37f2c1f0372a45554f1b89924fbb134fc24c3756efaedf11e07f599494e0eff9")
	dumbInitArm64Checksum   = env.NewDefault("DUMB_INIT_ARM64_CHECKSUM", "45b1bbf56cc03edda81e4220535a025bfe3ed6e93562222b9be4471005b3eeb3")
	dumbInitS390xChecksum   = env.NewDefault("DUMB_INIT_S390X_CHECKSUM", "8b3808c3c06d008b8f2eeb2789c7c99e0450b678d94fb50fd446b8f6a22e3a9d")
	dumbInitPpc64leChecksum = env.NewDefault("DUMB_INIT_PPC64LE_CHECKSUM", "88b02a3bd014e4c30d8d54389597adc4f5a36d1d6b49200b5a4f6a71026c2246")

	gitLfsVersion         = env.NewDefault("GIT_LFS_VERSION", "3.4.0")
	gitLfsAmd64Checksum   = env.NewDefault("GIT_LFS_AMD64_CHECKSUM", "60b7e9b9b4bca04405af58a2cd5dff3e68a5607c5bc39ee88a5256dd7a07f58c")
	gitLfsArm64Checksum   = env.NewDefault("GIT_LFS_ARM64_CHECKSUM", "aee90114f8f2eb5a11c1a6e9f1703a2bfcb4dc1fc4ba12a3a574c3a86952a5d0")
	gitLfsS390xChecksum   = env.NewDefault("GIT_LFS_S390X_CHECKSUM", "494191655c638f0a75d4d026ef58dc124fc4845361a144a0d1ade3986f2bb6e0")
	gitLfsPpc64leChecksum = env.NewDefault("GIT_LFS_PPC64LE_CHECKSUM", "1ed0277cf0ae309a4800971581ff169bbff5c865718250b11090f6a9386f7533")

	ubuntuVersion    = env.NewDefault("UBUNTU_VERSION", "20.04")
	alpine315Version = env.NewDefault("ALPINE_315_VERSION", "3.15.8")
	alpine316Version = env.NewDefault("ALPINE_316_VERSION", "3.16.5")
	alpine317Version = env.NewDefault("ALPINE_317_VERSION", "3.17.3")
	alpine318Version = env.NewDefault("ALPINE_318_VERSION", "3.18.2")
	ubiFIPSBaseImage = env.NewDefault("UBI_FIPS_BASE_IMAGE", "registry.gitlab.com/gitlab-org/gitlab-runner/ubi-fips-base")
	ubiFIPSVersion   = env.NewDefault("UBI_FIPS_VERSION", "8.8-860")
)

var checksumsFiles = map[string]string{
	"DOCKER_MACHINE": "/usr/bin/docker-machine",
	"DUMB_INIT":      "/usr/bin/dumb-init",
	"GIT_LFS":        "/tmp/git-lfs.tar.gz",
}

var flavorAliases = map[string][]string{
	"alpine3.18": {"alpine", "alpine3.18"},
}

type buildRunnerParams struct {
	flavor string
	archs  []string
}

type runnerBlueprintImpl struct {
	build.BlueprintBase

	dependencies []runnerImageFileDependency
	artifacts    []string
	params       buildRunnerParams
}

type runnerImageFileDependency struct {
	build.Component

	destination string
}

func (r runnerBlueprintImpl) Dependencies() []runnerImageFileDependency {
	return r.dependencies
}

func (r runnerBlueprintImpl) Artifacts() []build.Component {
	return lo.Map(r.artifacts, func(item string, _ int) build.Component {
		return build.NewDockerImage(item)
	})
}

func (r runnerBlueprintImpl) Data() buildRunnerParams {
	return r.params
}

func AssembleBuildRunner(flavor, targetArchs string) build.TargetBlueprint[runnerImageFileDependency, build.Component, buildRunnerParams] {
	archs := strings.Split(strings.ToLower(targetArchs), " ")

	flavors := flavorAliases[flavor]
	if len(flavors) == 0 {
		flavors = []string{flavor}
	}

	base := build.NewBlueprintBase(
		ci.RegistryImage,
		ci.RegistryAuthBundle,
		docker.BuilderEnvBundle,
		runnerImageName,
		dockerMachineVersion,
		dumbInitVersion,
		gitLfsVersion,
		ubuntuVersion,
		alpine315Version,
		alpine316Version,
		alpine317Version,
		alpine318Version,
		ubiFIPSBaseImage,
		ubiFIPSVersion,
		dockerMachineAmd64Checksum,
		dockerMachineArm64Checksum,
		dockerMachineS390xChecksum,
		dockerMachinePpc64leChecksum,
		dumbInitAmd64Checksum,
		dumbInitArm64Checksum,
		dumbInitS390xChecksum,
		dumbInitPpc64leChecksum,
		gitLfsAmd64Checksum,
		gitLfsArm64Checksum,
		gitLfsS390xChecksum,
		gitLfsPpc64leChecksum,
	)

	return runnerBlueprintImpl{
		BlueprintBase: base,
		dependencies:  assembleDependencies(archs),
		artifacts: tags(
			flavors,
			base.Env().Value(ci.RegistryImage),
			base.Env().Value(runnerImageName),
			build.RefTag(),
		),
		params: buildRunnerParams{
			flavor: flavor,
			archs:  archs,
		},
	}
}

func BuildRunner(blueprint build.TargetBlueprint[runnerImageFileDependency, build.Component, buildRunnerParams], publish bool) error {
	flavor := blueprint.Data().flavor
	archs := blueprint.Data().archs

	platform := flavor
	if strings.HasPrefix(platform, "alpine") {
		platform = "alpine"
	}

	if err := writeChecksums(archs, blueprint.Env()); err != nil {
		return fmt.Errorf("writing checksums: %w", err)
	}

	if err := copyDependencies(blueprint.Dependencies()); err != nil {
		return fmt.Errorf("copying dependencies: %w", err)
	}

	baseImagesFlavor := map[string]string{
		"ubuntu":        fmt.Sprintf("ubuntu:%s", blueprint.Env().Value(ubuntuVersion)),
		"alpine3.15":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine315Version)),
		"alpine3.16":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine316Version)),
		"alpine3.17":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine317Version)),
		"alpine3.18":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(alpine318Version)),
		"alpine-latest": "alpine:latest",
		"ubi-fips": fmt.Sprintf(
			"%s:%s",
			blueprint.Env().Value(ubiFIPSBaseImage),
			blueprint.Env().Value(ubiFIPSVersion),
		),
	}

	contextPath := filepath.Join(runnerHomeDir, platform)
	baseImage := baseImagesFlavor[flavor]

	return buildx(
		contextPath,
		baseImage,
		blueprint,
		publish,
	)
}

func writeChecksums(archs []string, env build.BlueprintEnv) error {
	checksumBinaries := map[string][]string{}
	checksums := map[string]string{}
	for _, v := range env.All() {
		value := env.Value(v)
		if value == "" || !strings.HasSuffix(v.Key, "_CHECKSUM") {
			continue
		}

		split := strings.Split(v.Key, "_")
		binaryName := strings.Join(split[:len(split)-2], "_")
		arch := strings.ToLower(split[len(split)-2])
		checksumBinaries[binaryName] = append(checksumBinaries[binaryName], arch)
		checksums[binaryName+"_"+arch] = value
	}

	for _, arch := range archs {
		var sb strings.Builder
		for binary, checksumArchs := range checksumBinaries {
			if !lo.Contains(checksumArchs, arch) {
				continue
			}

			checksumFile := checksumsFiles[binary]
			checksum := checksums[binary+"_"+arch]

			sb.WriteString(fmt.Sprintf("%s  %s\n", checksum, checksumFile))
		}

		checksumsFile := sb.String()
		fmt.Printf("Writing checksums for %s: \n%s", arch, checksumsFile)

		err := os.WriteFile(
			filepath.Join(runnerHomeDir, fmt.Sprintf("checksums-%s", arch)),
			[]byte(checksumsFile),
			0600,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyDependencies(deps []runnerImageFileDependency) error {
	for _, dep := range deps {
		from := dep.Value()
		to := dep.destination
		if err := sh.RunV("cp", from, to); err != nil {
			return fmt.Errorf("copying %s to %s: %w", from, to, err)
		}
	}

	return nil
}

func assembleDependencies(archs []string) []runnerImageFileDependency {
	installDeps := []string{
		filepath.Join(runnerHomeDir, "install-deps"),
		"dockerfiles/install_git_lfs",
	}

	copyMap := map[string][]string{
		"ubuntu":   installDeps,
		"alpine":   installDeps,
		"ubi-fips": installDeps,
	}

	for _, arch := range archs {
		debArch := arch
		if arch == "ppc64le" {
			debArch = "ppc64el"
		}

		checksumsFile := filepath.Join(runnerHomeDir, fmt.Sprintf("checksums-%s", arch))

		copyMap["ubuntu"] = append(
			copyMap["ubuntu"],
			checksumsFile,
			fmt.Sprintf("out/deb/gitlab-runner_%s.deb", debArch),
		)

		copyMap["alpine"] = append(
			copyMap["alpine"],
			checksumsFile,
			fmt.Sprintf("out/binaries/gitlab-runner-linux-%s", arch),
		)

		if arch == "amd64" {
			copyMap["ubi-fips"] = append(
				copyMap["ubi-fips"],
				checksumsFile,
				fmt.Sprintf("out/binaries/gitlab-runner-linux-%s-fips", arch),
				fmt.Sprintf("out/rpm/gitlab-runner_%s-fips.rpm", arch),
			)
		}
	}

	var dependencies []runnerImageFileDependency

	for to, fromFiles := range copyMap {
		for _, from := range fromFiles {
			dependencies = append(dependencies, runnerImageFileDependency{
				Component:   build.NewFile(from),
				destination: filepath.Join(runnerHomeDir, to, path.Base(from)),
			})
		}
	}

	return dependencies
}

func buildx(
	contextPath, baseImage string,
	blueprint build.TargetBlueprint[runnerImageFileDependency, build.Component, buildRunnerParams],
	publish bool,
) error {
	env := blueprint.Env()
	args := []string{
		"--build-arg", fmt.Sprintf("BASE_IMAGE=%s", baseImage),
		"--build-arg", fmt.Sprintf("DOCKER_MACHINE_VERSION=%s", env.Value(dockerMachineVersion)),
		"--build-arg", fmt.Sprintf("DUMB_INIT_VERSION=%s", env.Value(dumbInitVersion)),
		"--build-arg", fmt.Sprintf("GIT_LFS_VERSION=%s", env.Value(gitLfsVersion)),
	}
	args = append(args, lo.Map(blueprint.Artifacts(), func(tag build.Component, _ int) string {
		return fmt.Sprintf("--tag=%s", tag.Value())
	})...)

	dockerOS, err := sh.Output("docker", "version", "-f", "{{.Server.Os}}")
	if err != nil {
		return err
	}
	args = append(args, lo.Map(blueprint.Data().archs, func(arch string, _ int) string {
		return fmt.Sprintf("--platform=%s/%s", dockerOS, arch)
	})...)

	if publish {
		args = append(args, "--push")
	} else if len(blueprint.Data().archs) == 1 {
		args = append(args, "--load")
	} else {
		fmt.Println("Building image:")
	}

	builder := docker.NewBuilder(
		env.Value(docker.Host),
		env.Value(docker.CertPath),
	)
	defer func() {
		_ = builder.CleanupContext()
	}()

	if err := builder.SetupContext(); err != nil {
		return err
	}

	if publish {
		logout, err := builder.Login(
			env.Value(ci.RegistryUser),
			env.Value(ci.RegistryPassword),
			env.Value(ci.Registry),
		)
		if err != nil {
			return err
		}

		defer logout()
	}

	args = append(args, contextPath)

	return builder.Buildx(append([]string{"build"}, args...)...)
}

func tags(baseImages []string, registryImage, imageName, refTag string) []string {
	var tags []string

	image := registryImage
	if imageName != "" {
		image = fmt.Sprintf("%s/%s", registryImage, imageName)
	}

	for _, base := range baseImages {
		tags = append(tags, fmt.Sprintf("%s:%s-%s", image, base, refTag))
		if base == DefaultFlavor {
			tags = append(tags, fmt.Sprintf("%s:%s", image, refTag))
		}

		if build.IsLatest() {
			tags = append(tags, fmt.Sprintf("%s:%s", image, base))
			if base == DefaultFlavor {
				tags = append(tags, fmt.Sprintf("%s:latest", image))
			}
		}
	}

	return tags
}
