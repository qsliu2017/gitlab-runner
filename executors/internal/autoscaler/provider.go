package autoscaler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"

	googlecompute "gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
)

type provider struct {
	common.ExecutorProvider

	mu      sync.Mutex
	scalers map[string]*taskscaler.Taskscaler
}

func New(ep common.ExecutorProvider) common.ExecutorProvider {
	return &provider{
		ExecutorProvider: ep,
		scalers:          make(map[string]*taskscaler.Taskscaler),
	}
}

func (p *provider) init(ctx context.Context, config *common.RunnerConfig) (*taskscaler.Taskscaler, bool, error) {
	if config.Autoscaler == nil {
		logrus.Fatal("executor requires autoscaler config")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	scaler, ok := p.scalers[config.GetToken()]
	if ok {
		return scaler, false, nil
	}

	// convert toml settings to json for unmarshaling into provider settings
	var settings fleetingprovider.Settings
	{
		raw, err := config.Autoscaler.InstanceGroupSettings.JSON()
		if err != nil {
			return nil, false, fmt.Errorf("marshaling provider settings: %w", err)
		}

		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, false, fmt.Errorf("unmarshaling provider settings: %w", err)
		}
	}

	if config.Autoscaler.Plugin != "fleeting-plugin-googlecompute" {
		panic("only fleeting-plugin-googlecompute is supported at the moment")
	}

	// convert toml settings to json for unmarshaling into plugin settings
	// todo: This unmarshaling wouldn't need to happen here if we were using
	// the Plugin Run functionality. We only need to do it here because we're
	// using the 'googlecompute' module.
	var group *googlecompute.InstanceGroup
	{
		raw, err := config.Autoscaler.PluginConfig.JSON()
		if err != nil {
			return nil, false, fmt.Errorf("marshaling plugin config: %w", err)
		}

		if err := json.Unmarshal(raw, &group); err != nil {
			return nil, false, fmt.Errorf("unmarshaling plugin config: %w", err)
		}
	}

	options := []taskscaler.Option{
		taskscaler.WithCapacityPerInstance(config.Autoscaler.CapacityPerInstance),
		taskscaler.WithMaxUseCount(config.Autoscaler.MaxUseCount),
		taskscaler.WithMaxInstances(config.Autoscaler.MaxInstances),
		taskscaler.WithInstanceGroupSettings(settings),
	}

	scaler, err := taskscaler.New(ctx, group, options...)
	if err != nil {
		return nil, false, fmt.Errorf("creating taskscaler: %w", err)
	}

	p.scalers[config.GetToken()] = scaler

	return scaler, true, nil
}

func (p *provider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	scaler, fresh, err := p.init(context.Background(), config)
	if err != nil {
		// todo: init should probably be fatal?
		return nil, fmt.Errorf("initializing taskscaler: %w", err)
	}

	if fresh /* || todo: also detect config updates - based on last modified timestamp ? */ {
		var schedules []taskscaler.Schedule
		for _, schedule := range config.Autoscaler.Policy {
			schedules = append(schedules, taskscaler.Schedule{
				Periods:          schedule.Periods,
				Timezone:         schedule.Timezone,
				IdleCount:        schedule.IdleCount,
				IdleTime:         schedule.IdleTime,
				ScaleFactor:      schedule.ScaleFactor,
				ScaleFactorLimit: schedule.ScaleFactorLimit,
			})
		}
		scaler.ConfigureSchedule(schedules...)
	}

	available, potential := scaler.Capacity()

	if potential <= 0 || available <= 0 {
		return nil, fmt.Errorf("already at capacity, cannot accept")
	}

	if scaler.Schedule().IdleCount > 0 && available <= 0 {
		return nil, fmt.Errorf("already at capacity, cannot accept, allow on demand is disabled")
	}

	return &acqusitionRef{}, nil
}

func (p *provider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	acq, ok := data.(*acqusitionRef)
	if !ok {
		return
	}

	p.getRunnerTaskscaler(config).Release(acq.get())
}

func (p *provider) Create() common.Executor {
	e := p.ExecutorProvider.Create()
	if e == nil {
		return nil
	}

	return &executor{
		provider: p,
		Executor: e,
	}
}

func (p *provider) getRunnerTaskscaler(config *common.RunnerConfig) *taskscaler.Taskscaler {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.scalers[config.GetToken()]
}
