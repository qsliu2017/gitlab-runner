package env

import "gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"

type Variable struct {
	Key   string
	Value string
}

func (v Variable) String() string {
	return v.Value
}

func New(key string) Variable {
	return Variable{
		Key:   key,
		Value: mageutils.Env(key),
	}
}

func NewDefault(key, def string) Variable {
	return Variable{
		Key:   key,
		Value: mageutils.EnvOrDefault(key, def),
	}
}

func NewFallbackOrDefault(key, fallback, def string) Variable {
	return Variable{
		Key:   key,
		Value: mageutils.EnvFallbackOrDefault(key, fallback, def),
	}
}
