//go:build mage

package main

import (
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/samber/lo"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/docker"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
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
	dumbInitAmd64Checksum   = "e874b55f3279ca41415d290c512a7ba9d08f98041b28ae7c2acb19a545f1c4df"
	dumbInitArm64Checksum   = "b7d648f97154a99c539b63c55979cd29f005f88430fb383007fe3458340b795e"
	dumbInitS390xChecksum   = "47e4601b152fc6dcb1891e66c30ecc62a2939fd7ffd1515a7c30f281cfec53b7"
	dumbInitPpc64leChecksum = "3d15e80e29f0f4fa1fc686b00613a2220bc37e83a35283d4b4cca1fbd0a5609f"

	gitLfsVersion         = "3.3.0"
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

	defaultFlavor = "ubuntu"
	defaultArchs  = "amd64"
	defaultImage  = build.AppName

	runnerHomeDir = "dockerfiles/runner"
)

var checksums = map[string]string{
	"DOCKER_MACHINE_AMD64":   mageutils.EnvOrDefault("DOCKER_MACHINE_LINUX_AMD64_CHECKSUM", dockerMachineAmd64Checksum),
	"DOCKER_MACHINE_ARM64":   mageutils.EnvOrDefault("DOCKER_MACHINE_LINUX_ARM64_CHECKSUM", dockerMachineArm64Checksum),
	"DOCKER_MACHINE_S390X":   "", // No binary available yet for s390x, see https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551
	"DOCKER_MACHINE_PPC64LE": "", // No binary available

	"DUMB_INIT_AMD64":   mageutils.EnvOrDefault("DUMB_INIT_LINUX_AMD64_CHECKSUM", dumbInitAmd64Checksum),
	"DUMB_INIT_ARM64":   mageutils.EnvOrDefault("DUMB_INIT_LINUX_ARM64_CHECKSUM", dumbInitArm64Checksum),
	"DUMB_INIT_S390X":   mageutils.EnvOrDefault("DUMB_INIT_LINUX_S390X_CHECKSUM", dumbInitS390xChecksum),
	"DUMB_INIT_PPC64LE": mageutils.EnvOrDefault("DUMB_INIT_LINUX_PPC64LE_CHECKSUM", dumbInitPpc64leChecksum),

	"GIT_LFS_AMD64":   mageutils.EnvOrDefault("GIT_LFS_LINUX_AMD64_CHECKSUM", gitLfsAmd64Checksum),
	"GIT_LFS_ARM64":   mageutils.EnvOrDefault("GIT_LFS_LINUX_ARM64_CHECKSUM", gitLfsArm64Checksum),
	"GIT_LFS_S390X":   mageutils.EnvOrDefault("GIT_LFS_LINUX_S390X_CHECKSUM", gitLfsS390xChecksum),
	"GIT_LFS_PPC64LE": mageutils.EnvOrDefault("GIT_LFS_LINUX_PPC64LE_CHECKSUM", gitLfsPpc64leChecksum),
}

var checksumsFiles = map[string]string{
	"DOCKER_MACHINE": "/usr/bin/docker-machine",
	"DUMB_INIT":      "/usr/bin/dumb-init",
	"GIT_LFS":        "/tmp/git-lfs.tar.gz",
}

var baseImagesFlavor = map[string]string{
	"ubuntu":        fmt.Sprintf("ubuntu:%s", mageutils.EnvOrDefault("UBUNTU_VERSION", ubuntuVersion)),
	"alpine3.15":    fmt.Sprintf("alpine:%s", mageutils.EnvOrDefault("ALPINE_315_VERSION", alpine315Version)),
	"alpine3.16":    fmt.Sprintf("alpine:%s", mageutils.EnvOrDefault("ALPINE_316_VERSION", alpine316Version)),
	"alpine3.17":    fmt.Sprintf("alpine:%s", mageutils.EnvOrDefault("ALPINE_317_VERSION", alpine317Version)),
	"alpine3.18":    fmt.Sprintf("alpine:%s", mageutils.EnvOrDefault("ALPINE_318_VERSION", alpine318Version)),
	"alpine-latest": "alpine:latest",
	"ubi-fips": fmt.Sprintf(
		"%s:%s",
		mageutils.EnvOrDefault("UBI_FIPS_BASE_IMAGE", ubiFIPSBaseImage),
		mageutils.EnvOrDefault("UBI_FIPS_VERSION", ubiFIPSVersion),
	),
}

type Images mg.Namespace

func (i Images) BuildDefault() error {
	return i.Build(defaultFlavor, false, defaultArchs)
}

func (Images) Build(flavor string, publish bool, targetArchs string) error {
	archs := strings.Split(targetArchs, " ")

	//dockerMachineVersion := mageutils.EnvOrDefault("DOCKER_MACHINE_VERSION", dockerMachineVersion)
	//dumbInitVersion := mageutils.EnvOrDefault("DUMB_INIT_VERSION", dumbInitVersion)
	//gitLfsVersion := mageutils.EnvOrDefault("GIT_LFS_VERSION", gitLfsVersion)
	repository := mageutils.EnvOrDefault("CI_REGISTRY_IMAGE", defaultImage)

	flavorAliases := map[string][]string{
		"alpine3.18": {"alpine", "alpine3.18"},
	}

	baseImage := baseImagesFlavor[flavor]

	flavors := flavorAliases[flavor]
	if len(flavors) == 0 {
		flavors = []string{flavor}
	}

	platform := flavor
	if strings.HasPrefix(platform, "alpine") {
		platform = "alpine"
	}

	tags := tags(flavors, repository, build.RefTag())

	if err := writeChecksums(archs); err != nil {
		return fmt.Errorf("writing checksums: %w", err)
	}

	if err := copyDependencies(archs); err != nil {
		return fmt.Errorf("copying dependencies: %w", err)
	}

	contextPath := filepath.Join(runnerHomeDir, platform)

	return buildx(
		contextPath,
		baseImage,
		publish,
		archs,
		tags,
	)
}

func writeChecksums(archs []string) error {
	checksumBinaries := map[string]struct{}{}
	for k := range checksums {
		split := strings.Split(k, "_")
		binaryName := strings.Join(split[:len(split)-1], "_")
		checksumBinaries[binaryName] = struct{}{}
	}

	for _, arch := range archs {
		fmt.Println("Writing checksums for arch", arch)

		var sb strings.Builder
		for binary := range checksumBinaries {
			checksumFile := checksumsFiles[binary]
			checksum := checksums[fmt.Sprintf("%s_%s", binary, strings.ToUpper(arch))]

			sb.WriteString(fmt.Sprintf("%s  %s\n", checksum, checksumFile))
		}

		err := os.WriteFile(
			filepath.Join(runnerHomeDir, fmt.Sprintf("checksums-%s", arch)),
			[]byte(sb.String()),
			0600,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyDependencies(archs []string) error {
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

	for to, fromFiles := range copyMap {
		for _, from := range fromFiles {
			toPath := filepath.Join(runnerHomeDir, to, path.Base(from))
			if err := sh.RunV("cp", from, toPath); err != nil {
				return fmt.Errorf("copying %s to %s: %w", from, toPath, err)
			}
		}
	}

	return nil
}

func buildx(contextPath, baseImage string, publish bool, archs, tags []string) error {
	var args []string

	args = append(args, "--build-arg", fmt.Sprintf("DOCKER_MACHINE_VERSION=%s", dockerMachineVersion))
	args = append(args, "--build-arg", fmt.Sprintf("DUMB_INIT_VERSION=%s", dumbInitVersion))
	args = append(args, "--build-arg", fmt.Sprintf("GIT_LFS_VERSION=%s", gitLfsVersion))
	args = append(args, "--build-arg", fmt.Sprintf("BASE_IMAGE=%s", baseImage))

	args = append(args, lo.Map(tags, func(tag string, _ int) string {
		return fmt.Sprintf("--tag=%s", tag)
	})...)

	dockerOS, err := sh.Output("docker", "version", "-f", "{{.Server.Os}}")
	if err != nil {
		return err
	}
	args = append(args, lo.Map(archs, func(arch string, _ int) string {
		return fmt.Sprintf("--platform=%s/%s", dockerOS, arch)
	})...)

	if publish {
		args = append(args, "--push")
	} else if len(archs) == 1 {
		args = append(args, "--load")
	} else {
		fmt.Println("Building image:")
	}

	builder := docker.NewBuilder()
	defer builder.CleanupContext()

	if err := builder.SetupContext(); err != nil {
		return err
	}

	ciUser := os.Getenv("CI_REGISTRY_USER")
	ciPassword := os.Getenv("CI_REGISTRY_PASSWORD")
	if publish && ciUser != "" && ciPassword != "" {
		if err := builder.Login(ciUser, ciPassword, os.Getenv("CI_REGISTRY")); err != nil {
			return err
		}

		defer builder.Logout(os.Getenv("CI_REGISTRY"))
	}

	args = append(args, contextPath)

	return builder.Buildx(append([]string{"build"}, args...)...)
}

func tags(baseImages []string, repo, refTag string) []string {
	var tags []string
	for _, base := range baseImages {
		tags = append(tags, fmt.Sprintf("%s:%s-%s", repo, base, refTag))
		if base == defaultFlavor {
			tags = append(tags, fmt.Sprintf("%s:%s", repo, refTag))
		}

		if build.IsLatest() {
			tags = append(tags, fmt.Sprintf("%s:%s", repo, base))
			if base == defaultFlavor {
				tags = append(tags, fmt.Sprintf("%s:latest", repo))
			}
		}
	}

	return tags
}
