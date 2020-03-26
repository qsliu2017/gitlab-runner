package networks

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	docker_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

var errBuildNetworkExists = errors.New("build network is not empty")

type Manager interface {
	Create(ctx context.Context, networkMode string) (container.NetworkMode, error)
	Inspect(ctx context.Context) (types.NetworkResource, error)
	Cleanup(ctx context.Context) error
}

type manager struct {
	logger logrus.FieldLogger
	client docker_helpers.Client
	build  *common.Build

	networkMode  container.NetworkMode
	buildNetwork types.NetworkResource
	perBuild     bool
}

func NewManager(logger logrus.FieldLogger, dockerClient docker_helpers.Client, build *common.Build) Manager {
	return &manager{
		logger: logger,
		client: dockerClient,
		build:  build,
	}
}

func (m *manager) Create(ctx context.Context, networkMode string) (container.NetworkMode, error) {
	m.networkMode = container.NetworkMode(networkMode)
	m.perBuild = false

	if networkMode != "" {
		return m.networkMode, nil
	}

	if !m.build.IsFeatureFlagOn(featureflags.NetworkPerBuild) {
		return m.networkMode, nil
	}

	if m.buildNetwork.ID != "" {
		return "", errBuildNetworkExists
	}

	networkName := fmt.Sprintf("%s-job-%d-network", m.build.ProjectUniqueName(), m.build.ID)
	m.logger = m.logger.WithField("BuildNetworkName", networkName)

	m.logger.Debug("Creating build network")

	networkResponse, err := m.client.NetworkCreate(ctx, networkName, types.NetworkCreate{})
	if err != nil {
		return "", err
	}

	// Inspect the created network to save its details
	m.buildNetwork, err = m.client.NetworkInspect(ctx, networkResponse.ID)
	if err != nil {
		return "", err
	}

	m.logger = m.logger.WithField("BuildNetworkID", m.buildNetwork.ID)
	m.networkMode = container.NetworkMode(networkName)
	m.perBuild = true

	return m.networkMode, nil
}

func (m *manager) Inspect(ctx context.Context) (types.NetworkResource, error) {
	if m.perBuild != true {
		return types.NetworkResource{}, nil
	}

	m.logger.Debug("Inspect docker network")

	return m.client.NetworkInspect(ctx, m.buildNetwork.ID)
}

func (m *manager) Cleanup(ctx context.Context) error {
	if !m.build.IsFeatureFlagOn(featureflags.NetworkPerBuild) {
		return nil
	}

	if !m.perBuild {
		return nil
	}

	m.logger.Debug("Removing network")

	err := m.client.NetworkRemove(ctx, m.buildNetwork.ID)
	if err != nil {
		return fmt.Errorf("docker remove network %s: %w", m.buildNetwork.ID, err)
	}

	return nil
}
