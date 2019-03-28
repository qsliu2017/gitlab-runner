package volumes

import (
	"crypto/md5"
	"errors"
	"fmt"
	"path/filepath"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/volumes/parser"
)

type parserProvider interface {
	CreateParser() (parser.Parser, error)
}

type Manager interface {
	CreateUserVolumes(volumes []string) error
	CreateBuildVolume(jobsRootDir string, volumes []string) error
	VolumeBindings() []string
	CacheContainerIDs() []string
	TmpContainerIDs() []string
}

type DefaultManagerConfig struct {
	CacheDir        string
	FullProjectDir  string
	ProjectUniqName string
	GitStrategy     common.GitStrategy
	DisableCache    bool
}

type defaultManager struct {
	config DefaultManagerConfig

	logger           common.BuildLogger
	parserProvider   parserProvider
	containerManager ContainerManager

	volumeBindings    []string
	cacheContainerIDs []string
	tmpContainerIDs   []string
}

func NewDefaultManager(logger common.BuildLogger, pProvider parserProvider, cManager ContainerManager, config DefaultManagerConfig) Manager {
	return &defaultManager{
		config:            config,
		logger:            logger,
		parserProvider:    pProvider,
		containerManager:  cManager,
		volumeBindings:    make([]string, 0),
		cacheContainerIDs: make([]string, 0),
		tmpContainerIDs:   make([]string, 0),
	}
}

func (m *defaultManager) CreateUserVolumes(volumes []string) error {
	for _, volume := range volumes {
		err := m.addVolume(volume)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *defaultManager) addVolume(volume string) error {
	volumeParser, err := m.parserProvider.CreateParser()
	if err != nil {
		return err
	}

	parsedVolume, err := volumeParser.ParseVolume(volume)
	if err != nil {
		return err
	}

	switch parsedVolume.Len() {
	case 2:
		err = m.addHostVolume(parsedVolume)
	case 1:
		err = m.addCacheVolume(parsedVolume)
	}

	if err != nil {
		m.logger.Errorln("Failed to create container volume for", volume, err)
	}

	return err
}

func (m *defaultManager) addHostVolume(volume *parser.Volume) error {
	volume.Destination = m.getAbsoluteContainerPath(volume.Destination)
	m.appendVolumeBind(volume)

	return nil
}

func (m *defaultManager) getAbsoluteContainerPath(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}

	return filepath.Join(m.config.FullProjectDir, dir)
}

func (m *defaultManager) appendVolumeBind(volume *parser.Volume) {
	m.logger.Debugln(fmt.Sprintf("Using host-based %q for %q...", volume.Source, volume.Destination))
	m.volumeBindings = append(m.volumeBindings, volume.Definition())
}

func (m *defaultManager) addCacheVolume(volume *parser.Volume) error {
	volume.Destination = m.getAbsoluteContainerPath(volume.Destination)

	// disable cache for automatic container cache,
	// but leave it for host volumes (they are shared on purpose)
	if m.config.DisableCache {
		m.logger.Debugln(fmt.Sprintf("Container cache for %q is disabled", volume.Destination))

		return nil
	}

	hash := md5.Sum([]byte(volume.Destination))
	if m.config.CacheDir != "" {
		return m.createHostBasedCacheVolume(volume.Destination, hash)
	}

	return m.createContainerBasedCacheVolume(volume.Destination, hash)
}

func (m *defaultManager) createHostBasedCacheVolume(containerPath string, hash [md5.Size]byte) error {
	hostPath := filepath.Join(m.config.CacheDir, m.config.ProjectUniqName, fmt.Sprintf("%x", hash))
	hostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return err
	}

	m.appendVolumeBind(&parser.Volume{
		Source:      hostPath,
		Destination: containerPath,
	})

	return nil
}

func (m *defaultManager) createContainerBasedCacheVolume(containerPath string, hash [md5.Size]byte) error {
	containerName := fmt.Sprintf("%s-cache-%x", m.config.ProjectUniqName, hash)

	containerID := m.containerManager.FindExistingCacheContainer(containerName, containerPath)

	// create new cache container for that project
	if containerID == "" {
		var err error

		containerID, err = m.containerManager.CreateCacheContainer(containerName, containerPath)
		if err != nil {
			return err
		}
	}

	m.logger.Debugln(fmt.Sprintf("Using container %q as cache %q...", containerID, containerPath))
	m.cacheContainerIDs = append(m.cacheContainerIDs, containerID)

	return nil
}

func (m *defaultManager) CreateBuildVolume(jobsRootDir string, volumes []string) error {
	// Cache Git sources:
	// use a `jobsRootDir`
	if !filepath.IsAbs(jobsRootDir) || jobsRootDir == "/" {
		return errors.New("build directory needs to be absolute and non-root path")
	}

	isHostMounted, err := m.isHostMountedVolume(jobsRootDir, volumes)
	if err != nil {
		return err
	}

	if isHostMounted {
		// If builds directory is within a volume mounted manually by user
		// it will be added by CreateUserVolumes(), so nothing more to do
		// here
		return nil
	}

	if m.config.GitStrategy == common.GitFetch && !m.config.DisableCache {
		// create persistent cache container
		return m.addVolume(jobsRootDir)
	}

	// create temporary cache container
	id, err := m.containerManager.CreateCacheContainer("", jobsRootDir)
	if err != nil {
		return err
	}

	m.cacheContainerIDs = append(m.cacheContainerIDs, id)
	m.tmpContainerIDs = append(m.tmpContainerIDs, id)

	return nil
}

func (m *defaultManager) isHostMountedVolume(path string, volumes []string) (bool, error) {
	volumeParser, err := m.parserProvider.CreateParser()
	if err != nil {
		return false, err
	}

	isHostMounted, err := IsHostMountedVolume(volumeParser, path, volumes...)
	if err != nil {
		return false, err
	}

	return isHostMounted, nil
}

func (m *defaultManager) VolumeBindings() []string {
	return m.volumeBindings
}

func (m *defaultManager) CacheContainerIDs() []string {
	return m.cacheContainerIDs
}

func (m *defaultManager) TmpContainerIDs() []string {
	return append(m.tmpContainerIDs, m.containerManager.FailedContainerIDs()...)
}
