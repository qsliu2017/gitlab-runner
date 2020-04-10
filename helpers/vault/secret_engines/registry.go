package secret_engines

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
)

type ErrFactoryAlreadyRegistered struct {
	engineName string
}

func NewErrFactoryAlreadyRegistered(engineName string) *ErrFactoryAlreadyRegistered {
	return &ErrFactoryAlreadyRegistered{
		engineName: engineName,
	}
}

func (e *ErrFactoryAlreadyRegistered) Error() string {
	return fmt.Sprintf("factory for engine %q already registered", e.engineName)
}

func (e *ErrFactoryAlreadyRegistered) Is(err error) bool {
	_, ok := err.(*ErrFactoryAlreadyRegistered)

	return ok
}

type ErrFactoryNotRegistered struct {
	engineName string
}

func NewErrFactoryNotRegistered(engineName string) *ErrFactoryNotRegistered {
	return &ErrFactoryNotRegistered{
		engineName: engineName,
	}
}

func (e *ErrFactoryNotRegistered) Error() string {
	return fmt.Sprintf("factory for engine %q is not registered", e.engineName)
}

func (e *ErrFactoryNotRegistered) Is(err error) bool {
	_, ok := err.(*ErrFactoryNotRegistered)

	return ok
}

type Factory func(client vault.Client, path string) vault.SecretEngine

type factoryRegistry map[string]Factory

var reg factoryRegistry

func (r factoryRegistry) Register(engineName string, factory Factory) error {
	_, ok := r[engineName]
	if ok {
		return NewErrFactoryAlreadyRegistered(engineName)
	}

	r[engineName] = factory

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
