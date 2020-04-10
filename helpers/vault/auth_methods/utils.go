package auth_methods

import (
	"fmt"
)

type ErrMissingRequiredConfigurationKey struct {
	key string
}

func NewErrMissingRequiredConfigurationKey(key string) *ErrMissingRequiredConfigurationKey {
	return &ErrMissingRequiredConfigurationKey{
		key: key,
	}
}

func (e *ErrMissingRequiredConfigurationKey) Error() string {
	return fmt.Sprintf("missing required auth method configuration key %q", e.key)
}

func (e *ErrMissingRequiredConfigurationKey) Is(err error) bool {
	_, ok := err.(*ErrMissingRequiredConfigurationKey)
	return ok
}

func FilterAuthenticationData(requiredFields []string, allowedFields []string, data map[string]interface{}) (map[string]interface{}, error) {
	for _, required := range requiredFields {
		_, ok := data[required]
		if !ok {
			return nil, NewErrMissingRequiredConfigurationKey(required)
		}
	}

	newData := make(map[string]interface{})
	for _, allowed := range allowedFields {
		value, ok := data[allowed]
		if !ok {
			continue
		}
		newData[allowed] = value
	}

	return newData, nil
}
