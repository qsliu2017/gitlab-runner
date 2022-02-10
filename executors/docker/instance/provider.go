package instance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"

	googlecompute "gitlab.com/jobd/fleeting/fleeting-plugin-googlecompute"
	"gitlab.com/jobd/fleeting/fleeting/connector"
	"gitlab.com/jobd/fleeting/fleeting/provider"
	"gitlab.com/jobd/fleeting/taskscaler"
)

const initScript = `echo "init"
sudo iptables -A INPUT -p tcp -m tcp --dport 443 -j ACCEPT

sudo mkdir -p /etc/systemd/system/docker.service.d/
printf "[Service]\nExecStart=\nExecStart=/usr/bin/dockerd --registry-mirror=https://mirror.gcr.io -H tcp://0.0.0.0:443 --containerd=/var/run/containerd/containerd.sock --tlsverify --tlscacert /etc/docker/ca.pem --tlscert /etc/docker/server-cert.pem --tlskey /etc/docker/server-key.pem $DOCKER_OPTS" | sudo tee /etc/systemd/system/docker.service.d/10-machine.conf

echo "%s" | sudo tee /etc/docker/ca.pem
echo "%s" | sudo tee /etc/docker/server-cert.pem
echo "%s" | sudo tee /etc/docker/server-key.pem

sudo systemctl daemon-reload
sudo systemctl restart docker
`

type instanceProvider struct {
	common.ExecutorProvider

	mu      sync.Mutex
	scalers map[string]*taskscaler.Taskscaler
}

type acqusitionRef struct {
	mu  sync.Mutex
	key string
}

func (ref *acqusitionRef) Set(key string) {
	ref.mu.Lock()
	defer ref.mu.Unlock()

	ref.key = key
}

func (ref *acqusitionRef) Get() string {
	ref.mu.Lock()
	defer ref.mu.Unlock()

	return ref.key
}

func New() common.ExecutorProvider {
	provider := common.GetExecutorProvider("docker")
	if provider == nil {
		logrus.Panicln("Missing docker executor")
	}

	return &instanceProvider{
		ExecutorProvider: provider,
		scalers:          make(map[string]*taskscaler.Taskscaler),
	}
}

func (p *instanceProvider) init(ctx context.Context, config *common.RunnerConfig) (*taskscaler.Taskscaler, bool, error) {
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
	var settings provider.Settings
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
		taskscaler.WithInstanceUpFunc(func(id string, info provider.ConnectInfo) error {
			creds, err := generateCertificates([]string{info.ExternalAddr, info.InternalAddr, "localhost", "127.0.0.1"})
			if err != nil {
				panic(err)
			}

			key := sha256.Sum256([]byte(id))
			os.Mkdir(filepath.Join(os.TempDir(), hex.EncodeToString(key[:])), 0777)
			os.WriteFile(filepath.Join(os.TempDir(), hex.EncodeToString(key[:]), "ca.pem"), creds.ca, 0777)
			os.WriteFile(filepath.Join(os.TempDir(), hex.EncodeToString(key[:]), "cert.pem"), creds.client, 0777)
			os.WriteFile(filepath.Join(os.TempDir(), hex.EncodeToString(key[:]), "key.pem"), creds.key, 0777)

			return connector.Run(context.TODO(), info, connector.ConnectorOptions{
				RunOptions: connector.RunOptions{
					Command: fmt.Sprintf(initScript, string(creds.ca), string(creds.server), string(creds.key)),
					Stdout:  os.Stdout,
					Stderr:  os.Stderr,
				},
				UseExternalAddr: true,
			})
		}),
	}

	scaler, err := taskscaler.New(ctx, group, options...)
	if err != nil {
		return nil, false, fmt.Errorf("creating taskscaler: %w", err)
	}

	p.scalers[config.GetToken()] = scaler

	return scaler, true, nil
}

func (p *instanceProvider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	scaler, fresh, err := p.init(context.Background(), config)
	if err != nil {
		// todo: init should probably be fatal?
		return nil, fmt.Errorf("initializing taskscaler: %w", err)
	}

	if fresh /* || todo: also detect config updates */ {
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

func (p *instanceProvider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	acq, ok := data.(*acqusitionRef)
	if !ok {
		return
	}

	p.getRunnerTaskscaler(config).Release(acq.Get())
}

func (p *instanceProvider) Create() common.Executor {
	return &executor{provider: p}
}

func (p *instanceProvider) getRunnerTaskscaler(config *common.RunnerConfig) *taskscaler.Taskscaler {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.scalers[config.GetToken()]
}

func init() {
	common.RegisterExecutorProvider("docker+instance", New())
}
