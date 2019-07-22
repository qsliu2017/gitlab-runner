package common

import (
	"io"
	"strings"

	mock "github.com/stretchr/testify/mock"
)

type MockCollector struct {
	mock.Mock
	example string
}

func (c *MockCollector) Collect() io.Reader {
	return strings.NewReader(c.example)
}

func NewMockCollector(example string) *MockCollector {
	return &MockCollector{
		example: example,
	}
}
