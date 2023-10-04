package images

import (
	"fmt"
	"github.com/magefile/mage/sh"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/ci"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/env"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	dockerMachineVersion       = "v0.16.2-gitlab.21"
	dockerMachineAmd64Checksum = "a4e9a416f30406772e76c3b9e795121d5a7e677978923f96b7fb72f0d8354740"
	dockerMachineArm64Checksum = "124ceefbe1a1eec44eeb932edf9f85dab1e532d449f5e3e236faed5e8b19caba"

	dumbInitVersion         = "1.2.2"
	dumbInitAmd64Checksum   = "37f2c1f0372a45554f1b89924fbb134fc24c3756efaedf11e07f599494e0eff9"
	dumbInitArm64Checksum   = "45b1bbf56cc03edda81e4220535a025bfe3ed6e93562222b9be4471005b3eeb3"
	dumbInitS390xChecksum   = "8b3808c3c06d008b8f2eeb2789c7c99e0450b678d94fb50fd446b8f6a22e3a9d"
	dumbInitPpc64leChecksum = "88b02a3bd014e4c30d8d54389597adc4f5a36d1d6b49200b5a4f6a71026c2246"

	gitLfsVersion         = "3.4.0"
	gitLfsAmd64Checksum   = "60b7e9b9b4bca04405af58a2cd5dff3e68a5607c5bc39ee88a5256dd7a07f58c"
	gitLfsArm64Checksum   = "aee90114f8f2eb5a11c1a6e9f1703a2bfcb4dc1fc4ba12a3a574c3a86952a5d0"
	gitLfsS390xChecksum   = "494191655c638f0a75d4d026ef58dc124fc4845361a144a0d1ade3986f2bb6e0"
	gitLfsPpc64leChecksum = "1ed0277cf0ae309a4800971581ff169bbff5c865718250b11090f6a9386f7533"

	ubuntuVersion    = "20.04"
	alpine315Version = "3.15.8"
	alpine316Version = "3.16.5"
	alpine317Version = "3.17.3"
	alpine318Version = "3.18.2"

	ubiFIPSBaseImage = "registry.gitlab.com/gitlab-org/gitlab-runner/ubi-fips-base"
	ubiFIPSVersion   = "8.8-860"

	DefaultFlavor = "ubuntu"
	DefaultArchs  = "amd64"

	runnerHomeDir = "dockerfiles/runner"

	envDockerMachineVersion              = "DOCKER_MACHINE_VERSION"
	envDumbInitVersion                   = "DUMB_INIT_VERSION"
	envGitLfsVersion                     = "GIT_LFS_VERSION"
	envUbuntuVersion                     = "UBUNTU_VERSION"
	envAlpine315Version                  = "ALPINE_315_VERSION"
	envAlpine316Version                  = "ALPINE_316_VERSION"
	envAlpine317Version                  = "ALPINE_317_VERSION"
	envAlpine318Version                  = "ALPINE_318_VERSION"
	envUbiFipsBaseImage                  = "UBI_FIPS_BASE_IMAGE"
	envUbiFipsVersion                    = "UBI_FIPS_VERSION"
	envDockerMachineLinuxAmd64Checksum   = "DOCKER_MACHINE_AMD64_CHECKSUM"
	envDockerMachineLinuxArm64Checksum   = "DOCKER_MACHINE_ARM64_CHECKSUM"
	envDockerMachineLinuxS390xChecksum   = "DOCKER_MACHINE_S390X_CHECKSUM"
	envDockerMachineLinuxPpc64leChecksum = "DOCKER_MACHINE_PPC64LE_CHECKSUM"
	envDumbInitAmd64Checksum             = "DUMB_INIT_AMD64_CHECKSUM"
	envDumbInitArm64Checksum             = "DUMB_INIT_ARM64_CHECKSUM"
	envDumbInitS390xChecksum             = "DUMB_INIT_S390X_CHECKSUM"
	envDumbInitPpc64leChecksum           = "DUMB_INIT_PPC64LE_CHECKSUM"
	envGitLFSAmd64Checksum               = "GIT_LFS_AMD64_CHECKSUM"
	envGitLFSArm64Checksum               = "GIT_LFS_ARM64_CHECKSUM"
	envGitLFSS390XChecksum               = "GIT_LFS_S390X_CHECKSUM"
	envGitLFSPpc64leChecksum             = "GIT_LFS_PPC64LE_CHECKSUM"
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

type runnerDependency lo.Tuple2[string, string]

func (d runnerDependency) String() string {
	return d.A
}

type runnerBlueprintImpl struct {
	build.BlueprintBase

	dependencies []runnerDependency
	artifacts    []string
	params       buildRunnerParams
}

func (r runnerBlueprintImpl) Dependencies() []runnerDependency {
	return r.dependencies
}

func (r runnerBlueprintImpl) Artifacts() []build.StringArtifact {
	return lo.Map(r.artifacts, func(item string, _ int) build.StringArtifact {
		return build.StringArtifact(item)
	})
}

func (r runnerBlueprintImpl) Data() buildRunnerParams {
	return r.params
}

func AssembleBuildRunner(flavor, targetArchs string) build.TargetBlueprint[runnerDependency, build.StringArtifact, buildRunnerParams] {
	archs := strings.Split(strings.ToLower(targetArchs), " ")

	flavors := flavorAliases[flavor]
	if len(flavors) == 0 {
		flavors = []string{flavor}
	}

	return runnerBlueprintImpl{
		BlueprintBase: build.NewBlueprintBase(
			ci.RegistryImage,
			env.NewDefault(envDockerMachineVersion, dockerMachineVersion),
			env.NewDefault(envDumbInitVersion, dumbInitVersion),
			env.NewDefault(envGitLfsVersion, gitLfsVersion),
			env.NewDefault(envUbuntuVersion, ubuntuVersion),
			env.NewDefault(envAlpine315Version, alpine315Version),
			env.NewDefault(envAlpine316Version, alpine316Version),
			env.NewDefault(envAlpine317Version, alpine317Version),
			env.NewDefault(envAlpine318Version, alpine318Version),
			env.NewDefault(envUbiFipsBaseImage, ubiFIPSBaseImage),
			env.NewDefault(envUbiFipsVersion, ubiFIPSVersion),
			env.NewDefault(envDockerMachineLinuxAmd64Checksum, dockerMachineAmd64Checksum),
			env.NewDefault(envDockerMachineLinuxArm64Checksum, dockerMachineArm64Checksum),
			env.New(envDockerMachineLinuxS390xChecksum), // s390x and ppc64le are not being released
			env.New(envDockerMachineLinuxPpc64leChecksum),
			env.NewDefault(envDumbInitAmd64Checksum, dumbInitAmd64Checksum),
			env.NewDefault(envDumbInitArm64Checksum, dumbInitArm64Checksum),
			env.NewDefault(envDumbInitS390xChecksum, dumbInitS390xChecksum),
			env.NewDefault(envDumbInitPpc64leChecksum, dumbInitPpc64leChecksum),
			env.NewDefault(envGitLFSAmd64Checksum, gitLfsAmd64Checksum),
			env.NewDefault(envGitLFSArm64Checksum, gitLfsArm64Checksum),
			env.NewDefault(envGitLFSS390XChecksum, gitLfsS390xChecksum),
			env.NewDefault(envGitLFSPpc64leChecksum, gitLfsPpc64leChecksum),
		),
		dependencies: assembleDependencies(archs),
		artifacts:    tags(flavors, ci.RegistryImage.Value, build.RefTag()),
		params: buildRunnerParams{
			flavor: flavor,
			archs:  archs,
		},
	}
}

func BuildRunner(blueprint build.TargetBlueprint[runnerDependency, build.StringArtifact, buildRunnerParams], publish bool) error {
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
		"ubuntu":        fmt.Sprintf("ubuntu:%s", blueprint.Env().Value(envUbuntuVersion)),
		"alpine3.15":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(envAlpine315Version)),
		"alpine3.16":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(envAlpine316Version)),
		"alpine3.17":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(envAlpine317Version)),
		"alpine3.18":    fmt.Sprintf("alpine:%s", blueprint.Env().Value(envAlpine318Version)),
		"alpine-latest": "alpine:latest",
		"ubi-fips": fmt.Sprintf(
			"%s:%s",
			blueprint.Env().Value(envUbiFipsBaseImage),
			blueprint.Env().Value(envUbiFipsVersion),
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
	for _, v := range env {
		if v.Value == "" || !strings.HasSuffix(v.Key, "_CHECKSUM") {
			continue
		}

		split := strings.Split(v.Key, "_")
		binaryName := strings.Join(split[:len(split)-2], "_")
		arch := strings.ToLower(split[len(split)-2])
		checksumBinaries[binaryName] = append(checksumBinaries[binaryName], arch)
		checksums[binaryName+"_"+arch] = v.Value
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

func copyDependencies(deps []runnerDependency) error {
	for _, dep := range deps {
		from := dep.A
		to := dep.B
		if err := sh.RunV("cp", from, to); err != nil {
			return fmt.Errorf("copying %s to %s: %w", from, to, err)
		}
	}

	return nil
}

func assembleDependencies(archs []string) []runnerDependency {
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

	var dependencies []runnerDependency

	for to, fromFiles := range copyMap {
		for _, from := range fromFiles {
			dependencies = append(dependencies, runnerDependency{
				A: from,
				B: filepath.Join(runnerHomeDir, to, path.Base(from)),
			})
		}
	}

	return dependencies
}

func buildx(
	contextPath, baseImage string,
	blueprint build.TargetBlueprint[runnerDependency, build.StringArtifact, buildRunnerParams],
	publish bool,
) error {
	env := blueprint.Env()

	var args []string

	args = append(args, "--build-arg", fmt.Sprintf("BASE_IMAGE=%s", baseImage))
	args = append(args, "--build-arg", fmt.Sprintf("DOCKER_MACHINE_VERSION=%s", env.Value(envDockerMachineVersion)))
	args = append(args, "--build-arg", fmt.Sprintf("DUMB_INIT_VERSION=%s", env.Value(envDumbInitVersion)))
	args = append(args, "--build-arg", fmt.Sprintf("GIT_LFS_VERSION=%s", env.Value(envGitLfsVersion)))

	args = append(args, lo.Map(blueprint.Artifacts(), func(tag build.StringArtifact, _ int) string {
		return fmt.Sprintf("--tag=%s", tag)
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

	builder := docker.NewBuilder()
	defer builder.CleanupContext()

	if err := builder.SetupContext(); err != nil {
		return err
	}

	if publish {
		logout, err := builder.Login(
			env.Value(ci.EnvRegistryUser),
			env.Value(ci.EnvRegistryPassword),
			env.Value(ci.EnvRegistry),
		)
		if err != nil {
			return err
		}

		defer logout()
	}

	args = append(args, contextPath)

	return builder.Buildx(append([]string{"build"}, args...)...)
}

func tags(baseImages []string, repo, refTag string) []string {
	var tags []string
	for _, base := range baseImages {
		tags = append(tags, fmt.Sprintf("%s:%s-%s", repo, base, refTag))
		if base == DefaultFlavor {
			tags = append(tags, fmt.Sprintf("%s:%s", repo, refTag))
		}

		if build.IsLatest() {
			tags = append(tags, fmt.Sprintf("%s:%s", repo, base))
			if base == DefaultFlavor {
				tags = append(tags, fmt.Sprintf("%s:latest", repo))
			}
		}
	}

	return tags
}
