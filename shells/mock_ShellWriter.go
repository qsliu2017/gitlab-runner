// Code generated by mockery v1.0.0. DO NOT EDIT.

// This comment works around https://github.com/vektra/mockery/issues/155

package shells

import common "gitlab.com/gitlab-org/gitlab-runner/common"
import mock "github.com/stretchr/testify/mock"

// MockShellWriter is an autogenerated mock type for the ShellWriter type
type MockShellWriter struct {
	mock.Mock
}

// Absolute provides a mock function with given fields: path
func (_m *MockShellWriter) Absolute(path string) string {
	ret := _m.Called(path)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(path)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Cd provides a mock function with given fields: path
func (_m *MockShellWriter) Cd(path string) {
	_m.Called(path)
}

// CheckForErrors provides a mock function with given fields:
func (_m *MockShellWriter) CheckForErrors() {
	_m.Called()
}

// Command provides a mock function with given fields: command, arguments
func (_m *MockShellWriter) Command(command string, arguments ...string) {
	_va := make([]interface{}, len(arguments))
	for _i := range arguments {
		_va[_i] = arguments[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, command)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// Else provides a mock function with given fields:
func (_m *MockShellWriter) Else() {
	_m.Called()
}

// EmptyLine provides a mock function with given fields:
func (_m *MockShellWriter) EmptyLine() {
	_m.Called()
}

// EndIf provides a mock function with given fields:
func (_m *MockShellWriter) EndIf() {
	_m.Called()
}

// EnvVariableKey provides a mock function with given fields: name
func (_m *MockShellWriter) EnvVariableKey(name string) string {
	ret := _m.Called(name)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(name)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Error provides a mock function with given fields: fmt, arguments
func (_m *MockShellWriter) Error(fmt string, arguments ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, fmt)
	_ca = append(_ca, arguments...)
	_m.Called(_ca...)
}

// IfCmd provides a mock function with given fields: cmd, arguments
func (_m *MockShellWriter) IfCmd(cmd string, arguments ...string) {
	_va := make([]interface{}, len(arguments))
	for _i := range arguments {
		_va[_i] = arguments[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, cmd)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// IfCmdWithOutput provides a mock function with given fields: cmd, arguments
func (_m *MockShellWriter) IfCmdWithOutput(cmd string, arguments ...string) {
	_va := make([]interface{}, len(arguments))
	for _i := range arguments {
		_va[_i] = arguments[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, cmd)
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// IfDirectory provides a mock function with given fields: path
func (_m *MockShellWriter) IfDirectory(path string) {
	_m.Called(path)
}

// IfFile provides a mock function with given fields: file
func (_m *MockShellWriter) IfFile(file string) {
	_m.Called(file)
}

// Line provides a mock function with given fields: text
func (_m *MockShellWriter) Line(text string) {
	_m.Called(text)
}

// MkDir provides a mock function with given fields: path
func (_m *MockShellWriter) MkDir(path string) {
	_m.Called(path)
}

// MkTmpDir provides a mock function with given fields: name
func (_m *MockShellWriter) MkTmpDir(name string) string {
	ret := _m.Called(name)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(name)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Notice provides a mock function with given fields: fmt, arguments
func (_m *MockShellWriter) Notice(fmt string, arguments ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, fmt)
	_ca = append(_ca, arguments...)
	_m.Called(_ca...)
}

// SectionStart provides a mock function with given fields: name
func (_m *MockShellWriter) SectionStart(name string) {
	_m.Called(name)
}

// SectionEnd provides a mock function with given fields: name
func (_m *MockShellWriter) SectionEnd(name string) {
	_m.Called(name)
}

// Print provides a mock function with given fields: fmt, arguments
func (_m *MockShellWriter) Print(fmt string, arguments ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, fmt)
	_ca = append(_ca, arguments...)
	_m.Called(_ca...)
}

// RmDir provides a mock function with given fields: path
func (_m *MockShellWriter) RmDir(path string) {
	_m.Called(path)
}

// RmFile provides a mock function with given fields: path
func (_m *MockShellWriter) RmFile(path string) {
	_m.Called(path)
}

// TmpFile provides a mock function with given fields: name
func (_m *MockShellWriter) TmpFile(name string) string {
	ret := _m.Called(name)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(name)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Variable provides a mock function with given fields: variable
func (_m *MockShellWriter) Variable(variable common.JobVariable) {
	_m.Called(variable)
}

// Warning provides a mock function with given fields: fmt, arguments
func (_m *MockShellWriter) Warning(fmt string, arguments ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, fmt)
	_ca = append(_ca, arguments...)
	_m.Called(_ca...)
}
