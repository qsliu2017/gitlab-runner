package machine

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type machineProvider struct {
	name     string
	machines machinesDetails

	machineCommand docker.Machine

	// executorProvider stores a the provider for the executor that
	// will be used to run the builds
	executorProvider common.ExecutorProvider

	lock            sync.RWMutex
	acquireLock     sync.Mutex
	stuckRemoveLock sync.Mutex

	// metrics
	totalActions      *prometheus.CounterVec
	currentStatesDesc *prometheus.Desc
	creationHistogram prometheus.Histogram
}

func (m *machineProvider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	if config.Machine == nil || config.Machine.MachineName == "" {
		return nil, errors.New("missing Machine options")
	}

	// Lock updating machines, because two Acquires can be run at the same time
	m.acquireLock.Lock()
	defer m.acquireLock.Unlock()

	machineNames, err := m.loadMachineNames(config)
	if err != nil {
		return nil, fmt.Errorf("couldn't load machine names: %w", err)
	}

	// Schedule redundant machines for removal and get the list
	// of valid machine names
	machinesData, validMachineNames := m.removeRedundantMachines(machineNames, config)

	// Pre-create machines
	m.createMachines(config, machinesData)

	machinesData.writeDebugInformation()
	machinesData.Logger().
		WithField("minIdleCount", config.Machine.GetIdleCount()).
		WithField("maxMachines", config.Limit).
		WithField("time", time.Now()).
		Debugln("Docker Machine Details")

	// Try to find a free machine
	machine := m.findFreeMachine(false, validMachineNames...)
	if machine != nil {
		return machine, nil
	}

	if config.Machine.GetIdleCount() != 0 && machinesData.Idle == 0 {
		return nil, errors.New("no free idle machines that can process builds")
	}

	machinesData.Logger().
		Debugln("No free machine acquired")

	// Strange result, but fully valid. It means that a free machine was not acquired,
	// byt also no error was found. For example - `IdleCout` is set to `0` and a new machine
	// creation was scheduled.
	// A nil common.ExecutorData is not a problem here. Runner will try to find a free Idle
	// machine again, before using it for job execution. If still none will be available, it
	// will schedule - but this time blocking - a creation of a new one. If creation will fail
	// - the job will fail as well.

	return nil, nil
}

func (m *machineProvider) loadMachineNames(config *common.RunnerConfig) ([]string, error) {
	machines, err := m.machineCommand.List()
	if err != nil {
		return nil, fmt.Errorf("couldn't list docker machines: %w", err)
	}

	machines = append(machines, m.intermediateMachineList(machines)...)
	machines = filterMachineList(machines, machineFilter(config))

	return machines, nil
}

// intermediateMachineList returns a list of machines that might not yet be
// persisted on disk, these machines are the ones between being virtually
// created, and `docker-machine create` getting executed we populate this data
// set to overcome the race conditions related to not-full set of machines
// returned by `docker-machine ls -q`
func (m *machineProvider) intermediateMachineList(excludedMachines []string) []string {
	var excludedSet map[string]struct{}
	var intermediateMachines []string

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, details := range m.machines {
		if details.isPersistedOnDisk() {
			continue
		}

		// lazy init set, as most of times we don't create new machines
		if excludedSet == nil {
			excludedSet = make(map[string]struct{}, len(excludedMachines))
			for _, excludedMachine := range excludedMachines {
				excludedSet[excludedMachine] = struct{}{}
			}
		}

		if _, ok := excludedSet[details.Name]; ok {
			continue
		}

		intermediateMachines = append(intermediateMachines, details.Name)
	}

	return intermediateMachines
}

func (m *machineProvider) removeRedundantMachines(
	machineNames []string,
	config *common.RunnerConfig,
) (*machinesData, []string) {
	data := &machinesData{
		Runner: config.ShortDescription(),
	}

	validMachineNames := make([]string, 0, len(machineNames))
	for _, name := range machineNames {
		machine := m.getMachineDetails(name)
		machine.LastSeen = time.Now()

		err := m.isMachineRedundant(config, data, machine)
		if err != nil {
			err = m.scheduleMachineRemoval(machine.Name, err)
			if err != nil {
				machine.logger().
					WithError(err).
					Errorln("Machine removal failed")
			}
		} else {
			validMachineNames = append(validMachineNames, name)
		}

		data.Count(machine)
	}

	return data, validMachineNames
}

func (m *machineProvider) isMachineRedundant(
	config *common.RunnerConfig,
	data *machinesData,
	machine *machineDetails,
) error {
	if machine.State != machineStateIdle {
		return nil
	}

	if config.Machine.MaxBuilds > 0 && machine.UsedCount >= config.Machine.MaxBuilds {
		// Limit number of builds
		return errors.New("too many builds")
	}

	if config.Limit > 0 && data.Total() >= config.Limit {
		// Limit maximum number of machines
		return errors.New("too many machines")
	}

	if time.Since(machine.Used) > time.Second*time.Duration(config.Machine.GetIdleTime()) {
		if data.Idle >= config.Machine.GetIdleCount() {
			// Remove machine that are way over the idle time
			return errors.New("too many idle machines")
		}
	}

	return nil
}

func (m *machineProvider) scheduleMachineRemoval(machineName string, reason ...interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	machine := m.machines[machineName]
	if machine == nil {
		return errors.New("couldn't remove machine: machine not found")
	}

	machine.remove(reason...)

	machine.writeDebugInformation()
	machine.logger().
		WithField("now", time.Now()).
		Warningln("Requesting machine removal")

	go m.finalizeRemoval(machine)

	return nil
}

func (m *machineProvider) finalizeRemoval(machine *machineDetails) {
	for {
		err := m.removeMachine(machine)
		if err == nil {
			break
		}
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.machines, machine.Name)

	machine.logger().
		WithField("now", time.Now()).
		Infoln("Machine removed")

	m.totalActions.WithLabelValues("removed").Inc()
}

func (m *machineProvider) removeMachine(machine *machineDetails) error {
	if !m.machineCommand.Exist(machine.Name) {
		machine.logger().
			Warningln("Skipping machine removal, because it doesn't exist")
		return nil
	}

	// This code limits amount of removal of stuck machines to one machine per interval
	if machine.isStuckOnRemove() {
		m.stuckRemoveLock.Lock()
		defer m.stuckRemoveLock.Unlock()
	}

	machine.logger().
		Warningln("Stopping machine")

	err := m.machineCommand.Stop(machine.Name, machineStopCommandTimeout)
	if err != nil {
		machine.logger().
			WithError(err).
			Warningln("Error while stopping machine")
	}

	machine.logger().
		Warningln("Removing machine")

	err = m.machineCommand.Remove(machine.Name)
	if err != nil {
		machine.RetryCount++
		time.Sleep(removeRetryInterval)

		return err
	}

	return nil
}

func (m *machineProvider) getMachineDetails(name string) *machineDetails {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.machineDetailsThreadUnsafe(name)
}

// thread-unsafe
// Usage must be guarded with m.lock.Lock()
func (m *machineProvider) machineDetailsThreadUnsafe(name string) *machineDetails {
	machine, ok := m.machines[name]
	if !ok {
		machine = &machineDetails{
			Name:      name,
			Created:   time.Now(),
			Used:      time.Now(),
			LastSeen:  time.Now(),
			UsedCount: 1, // any machine that we find we mark as already used
			State:     machineStateIdle,
		}
		m.machines[name] = machine
	}

	return machine
}

func (m *machineProvider) createMachines(config *common.RunnerConfig, data *machinesData) {
	for {
		if data.Available() >= config.Machine.GetIdleCount() {
			// Limit maximum number of idle machines
			break
		}

		if config.Limit > 0 && data.Total() >= config.Limit {
			// Limit maximum number of machines
			break
		}

		m.scheduleMachineCreation(config, machineStateIdle)
		data.Creating++
	}
}

func (m *machineProvider) scheduleMachineCreation(
	config *common.RunnerConfig,
	state machineState,
) (*machineDetails, chan error) {
	name := newMachineName(config)

	machine := m.acquireMachineDetails(name)
	machine.create()

	errCh := make(chan error, 1)
	go m.asynchronouslyCreateMachine(config.Machine, state, machine, errCh)

	return machine, errCh
}

func (m *machineProvider) acquireMachineDetails(name string) *machineDetails {
	m.lock.Lock()
	defer m.lock.Unlock()

	machine := m.machineDetailsThreadUnsafe(name)
	if machine.isUsed() {
		return nil
	}

	machine.acquire()

	return machine
}

func (m *machineProvider) asynchronouslyCreateMachine(
	config *common.DockerMachine,
	state machineState,
	machine *machineDetails,
	errCh chan error,
) {
	started := time.Now()

	err := m.machineCommand.Create(config.MachineDriver, machine.Name, config.MachineOptions...)
	for i := 0; i < 3 && err != nil; i++ {
		machine.RetryCount++
		machine.logger().
			WithError(err).
			Warningln("Machine creation failed, trying to provision")

		time.Sleep(provisionRetryInterval)

		err = m.machineCommand.Provision(machine.Name)
	}

	if err != nil {
		machine.logger().
			WithField("time", time.Since(started)).
			WithError(err).
			Errorln("Machine creation failed, trying to remove")

		removeErr := m.scheduleMachineRemoval(machine.Name, "Failed to create")
		if removeErr != nil {
			machine.logger().
				WithError(removeErr).
				Errorln("Machine removal failed")
		}

		errCh <- err

		return
	}

	machine.State = state
	machine.Used = time.Now()

	creationTime := time.Since(started)
	m.creationHistogram.Observe(creationTime.Seconds())
	m.totalActions.WithLabelValues("created").Inc()

	machine.logger().
		WithFields(logrus.Fields{
			"duration": creationTime,
			"now":      time.Now(),
		}).
		Infoln("Machine created")

	errCh <- nil
}

func (m *machineProvider) findFreeMachine(skipCache bool, machineNames ...string) (details *machineDetails) {
	numberOfMachines := len(machineNames)

	// Enumerate all machines in reverse order, to always take the newest machines first
	for idx := range machineNames {
		name := machineNames[numberOfMachines-idx-1]
		machine := m.acquireMachineDetails(name)
		if machine == nil {
			continue
		}

		// Check if node is running
		canConnect := m.machineCommand.CanConnect(name, skipCache)
		if !canConnect {
			err := m.scheduleMachineRemoval(name, "machine is unavailable")
			if err != nil {
				machine.logger().
					WithError(err).
					Errorln("Machine removal failed")
			}

			continue
		}

		return machine
	}

	return nil
}

func (m *machineProvider) Use(
	config *common.RunnerConfig,
	data common.ExecutorData,
) (common.RunnerConfig, common.ExecutorData, error) {
	var newData common.ExecutorData

	// Find a new machine
	machine, _ := data.(*machineDetails)
	if machine == nil || !machine.canBeUsed() || !m.machineCommand.CanConnect(machine.Name, true) {
		var err error
		machine, err = m.retryFindOrCreateMachineForUse(config)
		if err != nil {
			return common.RunnerConfig{}, nil, fmt.Errorf("couldn't find free or create a new machine: %w", err)
		}

		// Return details only if this is a new instance
		newData = machine
	}

	// Get machine credentials
	dockerCredentials, err := m.machineCommand.Credentials(machine.Name)
	if err != nil {
		if newData != nil {
			m.Release(config, newData)
		}

		return common.RunnerConfig{}, nil, fmt.Errorf("couldn't get credentials for machine: %w", err)
	}

	// Create shallow copy of config and store docker credentials in it
	newConfig := *config
	newConfig.Docker = &common.DockerConfig{}
	if config.Docker != nil {
		*newConfig.Docker = *config.Docker
	}
	newConfig.Docker.Credentials = dockerCredentials

	machine.use()

	m.totalActions.WithLabelValues("used").Inc()

	return newConfig, newData, nil
}

func (m *machineProvider) retryFindOrCreateMachineForUse(config *common.RunnerConfig) (*machineDetails, error) {
	var details *machineDetails
	var err error

	// Try to find a machine
	for i := 0; i < 3; i++ {
		details, err = m.findOrCreateMachineForUse(config)
		if err == nil {
			break
		}

		time.Sleep(provisionRetryInterval)
	}

	return details, err
}

func (m *machineProvider) findOrCreateMachineForUse(config *common.RunnerConfig) (*machineDetails, error) {
	machines, err := m.loadMachineNames(config)
	if err != nil {
		return nil, fmt.Errorf("couldn't load available machine names: %w", err)
	}

	machine := m.findFreeMachine(true, machines...)
	if machine == nil {
		var errCh chan error
		machine, errCh = m.scheduleMachineCreation(config, machineStateAcquired)

		err = <-errCh
		if err != nil {
			return nil, fmt.Errorf("couldn't create machine: %w", err)
		}
	}

	return machine, nil
}

func (m *machineProvider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	// Release machine
	details, ok := data.(*machineDetails)
	if ok {
		// Mark last used time when is Used
		if details.State == machineStateUsed {
			details.Used = time.Now()
		}

		// Remove machine if we already used it
		if config != nil && config.Machine != nil &&
			config.Machine.MaxBuilds > 0 && details.UsedCount >= config.Machine.MaxBuilds {
			err := m.scheduleMachineRemoval(details.Name, "Too many builds")
			if err == nil {
				return
			}

			details.logger().
				WithError(err).
				Errorln("Machine removal failed")
		}
		details.State = machineStateIdle
	}
}

func (m *machineProvider) CanCreate() bool {
	return m.executorProvider.CanCreate()
}

func (m *machineProvider) GetFeatures(features *common.FeaturesInfo) error {
	return m.executorProvider.GetFeatures(features)
}

func (m *machineProvider) GetDefaultShell() string {
	return m.executorProvider.GetDefaultShell()
}

func (m *machineProvider) Create() common.Executor {
	return &machineExecutor{
		machineProvider:  m,
		executorProvider: m.executorProvider,
	}
}

func newMachineProvider(name, executor string) *machineProvider {
	executorProvider := common.GetExecutorProvider(executor)
	if executorProvider == nil {
		logrus.Panicln("Missing", executor)
	}

	return &machineProvider{
		name:             name,
		machines:         make(machinesDetails),
		machineCommand:   docker.NewMachineCommand(),
		executorProvider: executorProvider,
		totalActions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "gitlab_runner_autoscaling_actions_total",
				Help: "The total number of actions executed by the provider.",
				ConstLabels: prometheus.Labels{
					"executor": name,
				},
			},
			[]string{"action"},
		),
		currentStatesDesc: prometheus.NewDesc(
			"gitlab_runner_autoscaling_machine_states",
			"The current number of machines per state in this provider.",
			[]string{"state"},
			prometheus.Labels{
				"executor": name,
			},
		),
		creationHistogram: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "gitlab_runner_autoscaling_machine_creation_duration_seconds",
				Help:    "Histogram of machine creation time.",
				Buckets: prometheus.ExponentialBuckets(30, 1.25, 10),
				ConstLabels: prometheus.Labels{
					"executor": name,
				},
			},
		),
	}
}
