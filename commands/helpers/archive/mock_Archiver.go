// Code generated by mockery v2.28.2. DO NOT EDIT.

package archive

import (
	context "context"
	fs "io/fs"

	mock "github.com/stretchr/testify/mock"
)

// MockArchiver is an autogenerated mock type for the Archiver type
type MockArchiver struct {
	mock.Mock
}

// Archive provides a mock function with given fields: ctx, files
func (_m *MockArchiver) Archive(ctx context.Context, files map[string]fs.FileInfo) error {
	ret := _m.Called(ctx, files)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, map[string]fs.FileInfo) error); ok {
		r0 = rf(ctx, files)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewMockArchiver interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockArchiver creates a new instance of MockArchiver. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockArchiver(t mockConstructorTestingTNewMockArchiver) *MockArchiver {
	mock := &MockArchiver{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
