package referees

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_CreateReferees(t *testing.T) {
	fakeMockExecutor := func(t *testing.T) (interface{}, func(t mock.TestingT) bool) {
		return struct{}{}, func(t mock.TestingT) bool { return false }
	}

	mockExecutor := func(t *testing.T) (interface{}, func(t mock.TestingT) bool) {
		m := new(MockRefereeExecutor)
		m.On("GetHostname").Return("runner-1234").Maybe()
		return m, m.AssertExpectations
	}

	testCases := map[string]struct {
		mockExecutor     func(t *testing.T) (interface{}, func(t mock.TestingT) bool)
		config           *Config
		expectedReferees []Referee
	}{
		"Executor doesn't support any referee": {
			mockExecutor:     fakeMockExecutor,
			config:           &Config{Metrics: &MetricsRefereeConfig{QueryInterval: 0}},
			expectedReferees: nil,
		},
		"Executor supports metrics referee": {
			mockExecutor:     mockExecutor,
			config:           &Config{Metrics: &MetricsRefereeConfig{QueryInterval: 0}},
			expectedReferees: []Referee{&MetricsReferee{}},
		},
		"No config provided": {
			mockExecutor:     mockExecutor,
			config:           nil,
			expectedReferees: nil,
		},
	}

	for testName, test := range testCases {
		t.Run(testName, func(t *testing.T) {
			logger := logrus.WithField("test", t.Name())

			executor, assertExpectations := test.mockExecutor(t)
			defer assertExpectations(t)

			referees := CreateReferees(executor, test.config, logger)

			if test.expectedReferees == nil {
				assert.Nil(t, referees)
				return
			}

			assert.Len(t, referees, len(test.expectedReferees))
			for i, expectedReferee := range test.expectedReferees {
				referee := referees[i]
				assert.IsType(t, expectedReferee, referees[i])
				assert.Equal(t, "runner-1234", referee.(*MetricsReferee).hostname)
			}
		})
	}
}
