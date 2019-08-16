package referees

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/labkit/log"
)

const defaultNetworkRefereeQueryTimeout = 10 * time.Second

type NetworkReferee struct {
	client       *elasticsearch.Client
	index        string
	queryTimeout time.Duration
	*BaseReferee
}

type NetworkRefereeConfig struct {
	ElasticsearchAddresses []string `toml:"elasticsearch_addresses,omitempty" json:"elasticsearch_addresses" long:"elasticsearch_addresses" env:"ELASTICSEARCH_ADDRESSES" description:"Elasticsearch node addresses"`
	ElasticsearchCloudID   string   `toml:"elasticsearch_cloud_id,omitempty" json:"elasticsearch_cloud_id" long:"elasticsearch_cloud_id" env:"ELASTICSEARCH_CLOUD_ID" description:"Elasticsearch Cloud ID (overrides node addresses)"`
	ElasticsearchUsername  string   `toml:"elasticsearch_username,omitempty" json:"elasticsearch_username" long:"elasticsearch_username" env:"ELASTICSEARCH_USERNAME" description:"Elasticsearch username"`
	ElasticsearchPassword  string   `toml:"elasticsearch_password,omitempty" json:"elasticsearch_password" long:"elasticsearch_password" env:"ELASTICSEARCH_PASSWORD" description:"Elasticsearch password"`
	ElasticsearchAPIKey    string   `toml:"elasticsearch_api_key,omitempty" json:"elasticsearch_api_key" long:"elasticsearch_api_key" env:"ELASTICSEARCH_API_KEY" description:"Elasticsearch API key (overrides username and password)"`
	ElasticsearchIndex     string   `toml:"elasticsearch_index,omitempty" json:"elasticsearch_index" long:"elasticsearch_index" env:"ELASTICSEARCH_INDEX" description:"Elasticsearch index"`
}

func (nr *NetworkReferee) ArtifactBaseName() string {
	return "network_referee.json"
}

func (nr *NetworkReferee) ArtifactType() string {
	return "network_referee"
}

func (nr *NetworkReferee) ArtifactFormat() string {
	return "gzip"
}

type missingConfigError struct {
	key string
}

func (m missingConfigError) Error() string {
	return fmt.Sprintf("%q not provided", m.key)
}

func (nr *NetworkReferee) Execute(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
) ([]byte, error) {
	startTime = startTime.UTC()
	endTime = endTime.UTC()
	queryLogger := nr.logger.WithFields(logrus.Fields{
		"start": startTime,
		"end":   endTime,
	})

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"match": map[string]interface{}{
							"fields.hostname": nr.hostname,
						},
					},
					map[string]interface{}{
						"range": map[string]interface{}{
							"@timestamp": map[string]interface{}{
								"gte": startTime.Format(time.RFC3339),
								"lte": endTime.Format(time.RFC3339),
							},
						},
					},
				},
			},
		},
	}

	var queryBody bytes.Buffer
	err := json.NewEncoder(&queryBody).Encode(query)
	if err != nil {
		queryLogger.WithError(err).Error("Failed to encode the query for elasticsearch")
		return nil, err
	}

	response, err := nr.client.Search(
		nr.client.Search.WithIndex(nr.index),
		nr.client.Search.WithBody(&queryBody),
		nr.client.Search.WithTimeout(nr.queryTimeout),
		nr.client.Search.WithContext(ctx),
	)

	if err != nil {
		queryLogger.WithError(err).Error("Failed to execute query on elasticsearch")
		return nil, err
	}

	defer response.Body.Close()

	if response.IsError() {
		err := fmt.Errorf("bad elasticsearch response: %s", response.Status())
		queryLogger.WithError(err).Error("Elasticsearch response indicates failure")
		return nil, err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		queryLogger.WithError(err).Error("Failed to read elasticsearch response")
		return nil, err
	}

	return body, nil
}

func newNetworkReferee(base *BaseReferee, config *Config) Referee {
	logger := log.WithField("referee", "network")
	if config.Network == nil {
		return nil
	}

	err := validateConfig(config.Network)
	if err != nil {
		logger.WithError(err).Error("Elasticsearch configuration is not valid")
		return nil
	}

	elasticConfig := elasticsearch.Config{
		Addresses: config.Network.ElasticsearchAddresses,
		CloudID:   config.Network.ElasticsearchCloudID,
		Username:  config.Network.ElasticsearchUsername,
		Password:  config.Network.ElasticsearchPassword,
		APIKey:    config.Network.ElasticsearchAPIKey,
	}

	client, err := elasticsearch.NewClient(elasticConfig)
	if err != nil {
		logger.WithError(err).Error("Failed to create elasticsearch client")
		return nil
	}

	return &NetworkReferee{
		client:       client,
		index:        config.Network.ElasticsearchIndex,
		queryTimeout: defaultNetworkRefereeQueryTimeout,
		BaseReferee:  base,
	}
}

func validateConfig(config *NetworkRefereeConfig) error {
	if len(config.ElasticsearchAddresses) == 0 && config.ElasticsearchCloudID == "" {
		return missingConfigError{key: "elasticsearch_addresses/elasticsearch_cloud_id"}
	}

	if config.ElasticsearchAPIKey == "" {
		if config.ElasticsearchUsername == "" || config.ElasticsearchPassword == "" {
			return missingConfigError{key: "elasticsearch_username/elasticsearch_password"}
		}
	}

	if config.ElasticsearchIndex == "" {
		return missingConfigError{key: "elasticsearch_index"}
	}

	return nil
}
