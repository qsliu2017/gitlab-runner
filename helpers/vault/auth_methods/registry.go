package auth_methods

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

type ErrFactoryAlreadyRegistered struct {
	authName string
}

func NewErrFactoryAlreadyRegistered(authName string) *ErrFactoryAlreadyRegistered {
	return &ErrFactoryAlreadyRegistered{
		authName: authName,
	}
}

func (e *ErrFactoryAlreadyRegistered) Error() string {
	return fmt.Sprintf("factory for auth method %q already registered", e.authName)
}

func (e *ErrFactoryAlreadyRegistered) Is(err error) bool {
	_, ok := err.(*ErrFactoryAlreadyRegistered)

	return ok
}

type ErrFactoryNotRegistered struct {
	authName string
}

func NewErrFactoryNotRegistered(authName string) *ErrFactoryNotRegistered {
	return &ErrFactoryNotRegistered{
		authName: authName,
	}
}

func (e *ErrFactoryNotRegistered) Error() string {
	return fmt.Sprintf("factory for auth method %q is not registered", e.authName)
}

func (e *ErrFactoryNotRegistered) Is(err error) bool {
	_, ok := err.(*ErrFactoryNotRegistered)

	return ok
}

type Factory func(path string, data map[string]interface{}) (vault.AuthMethod, error)

type factoryRegistry map[string]Factory

var reg factoryRegistry

func (r factoryRegistry) Register(authName string, factory Factory) error {
	_, ok := r[authName]
	if ok {
		return NewErrFactoryAlreadyRegistered(authName)
	}

	r[authName] = factory

	return nil
}

func (r factoryRegistry) Get(engineName string) (Factory, error) {
	factory, ok := r[engineName]
	if !ok {
		return nil, NewErrFactoryNotRegistered(engineName)
	}

	return factory, nil
}

func MustRegisterFactory(engineName string, factory Factory) {
	err := factoryRegistryInstance().Register(engineName, factory)
	if err != nil {
		panic(fmt.Sprintf("registering factory: %v", err))
	}
}

func factoryRegistryInstance() factoryRegistry {
	if reg == nil {
		reg = make(factoryRegistry)
	}

	return reg
}

func GetFactory(engineName string) (Factory, error) {
	return factoryRegistryInstance().Get(engineName)
}
