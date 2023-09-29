package docker

import (
	"fmt"
	"github.com/magefile/mage/sh"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
	"os"
)

const (
	defaultBuilderName = "buildx-builder"
	defaultContextName = "docker-buildx"
)

type Builder struct {
	builderName string
	contextName string
}

func NewBuilder() *Builder {
	return &Builder{
		builderName: defaultBuilderName,
		contextName: defaultContextName,
	}
}

func (b *Builder) Docker(args ...string) error {
	return sh.RunWithV(
		map[string]string{
			"DOCKER_CLI_EXPERIMENTAL": "true",
		},
		"docker",
		args...,
	)
}

func (b *Builder) Buildx(args ...string) error {
	return b.Docker(append([]string{"buildx"}, args...)...)
}

func (b *Builder) CleanupContext() error {
	// In the old script this output was supressed but let's see if there's reason to do so
	// might contain valuable info

	if err := b.Buildx("rm", b.builderName); err != nil {
		return err
	}

	return b.Docker("context", "rm", "-f", b.contextName)
}

func (b *Builder) SetupContext() error {
	// We need the context to not exist either way. If we don't clean it up, we just need to rerun the script
	// since it gets deleted in case of an error anyways. There are also some other edge cases where it's not being cleaned up
	// properly so this makes the building of images more consistent and less error prone
	if err := b.CleanupContext(); err != nil {
		fmt.Println("Error cleaning up context:", err)
	}

	// In order for `docker buildx create` to work, we need to replace DOCKER_HOST with a Docker context.
	// Otherwise, we get the following error:
	// > could not create a builder instance with TLS data loaded from environment.

	docker := fmt.Sprintf("host=%s", mageutils.EnvOrDefault("DOCKER_HOST", "unix:///var/run/docker.sock"))
	dockerCertPath := os.Getenv("DOCKER_CERT_PATH")
	if dockerCertPath != "" {
		docker = fmt.Sprintf(
			"host=%s,ca=%s/ca.pem,cert=%s/cert.pem,key=%s/key.pem",
			os.Getenv("DOCKER_HOST"),
			dockerCertPath,
			dockerCertPath,
			dockerCertPath,
		)
	}

	if err := b.Docker(
		"context", "create", b.contextName,
		"--default-stack-orchestrator", "swarm",
		"--description", "Temporary buildx Docker context",
		"--docker", docker,
	); err != nil {
		return err
	}

	return b.Buildx("create", "--use", "--name", b.builderName, b.contextName)
}

func (b *Builder) Login(username, password, registry string) error {
	loginCmd := fmt.Sprintf("echo %s | docker login --username %s --password-stdin %s", password, username, registry)
	return sh.RunV("sh", "-c", loginCmd)
}

func (b *Builder) Logout(registry string) error {
	return b.Docker("logout", registry)
}
