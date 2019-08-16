package referees

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type BaseReferee struct {
	hostname string
	logger   logrus.FieldLogger
}

type Referee interface {
	Execute(
		ctx context.Context,
		startTime time.Time,
		endTime time.Time,
	) ([]byte, error)
	ArtifactBaseName() string
	ArtifactType() string
	ArtifactFormat() string
}

type RefereeExecutor interface {
	GetHostname() string
}

type refereeFactory func(referee *BaseReferee, config *Config) Referee

type Config struct {
	Metrics *MetricsRefereeConfig `toml:"metrics,omitempty" json:"metrics" namespace:"metrics"`
	Network *NetworkRefereeConfig `toml:"network,omitempty" json:"network" namespace:"network"`
}

var refereeFactories = map[string]refereeFactory{
	"metrics": newMetricsReferee,
	"network": newNetworkReferee,
}

func CreateReferees(executor interface{}, config *Config, log logrus.FieldLogger) []Referee {
	if config == nil {
		log.Debug("No referees configured")
		return nil
	}

	// see if executor supports refereeing
	refereed, ok := executor.(RefereeExecutor)
	if !ok {
		log.Info("Executor not supported")
		return nil
	}

	hostname := refereed.GetHostname()

	var referees []Referee
	for name, factory := range refereeFactories {
		base := &BaseReferee{
			hostname: hostname,
			logger:   log.WithField("referee", name),
		}

		referee := factory(base, config)
		if referee != nil {
			referees = append(referees, referee)
		}
	}

	return referees
}
