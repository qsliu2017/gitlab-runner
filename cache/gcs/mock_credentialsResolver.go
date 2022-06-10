// Code generated by mockery v1.1.0. DO NOT EDIT.

package gcs

import (
	mock "github.com/stretchr/testify/mock"
	common "gitlab.com/gitlab-org/gitlab-runner/common"
)

// mockCredentialsResolver is an autogenerated mock type for the credentialsResolver type
type mockCredentialsResolver struct {
	mock.Mock
}

// Credentials provides a mock function with given fields:
func (_m *mockCredentialsResolver) Credentials() *common.CacheGCSCredentials {
	ret := _m.Called()

	var r0 *common.CacheGCSCredentials
	if rf, ok := ret.Get(0).(func() *common.CacheGCSCredentials); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*common.CacheGCSCredentials)
		}
	}

	return r0
}

// Resolve provides a mock function with given fields:
func (_m *mockCredentialsResolver) Resolve() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SignBytesFunc provides a mock function with given fields:
func (_m *mockCredentialsResolver) SignBytesFunc() func([]byte) ([]byte, error) {
	ret := _m.Called()

	var r0 func([]byte) ([]byte, error)
	if rf, ok := ret.Get(0).(func() func([]byte) ([]byte, error)); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func([]byte) ([]byte, error))
		}
	}

	return r0
}
