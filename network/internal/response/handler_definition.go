package response

import (
	"github.com/sirupsen/logrus"
)

type HandlerFn func(log logrus.FieldLogger) interface{}

func IdentityHandlerFn(value interface{}) HandlerFn {
	return func(_ logrus.FieldLogger) interface{} {
		return value
	}
}

type handlerDefinition struct {
	code        int
	logArgument string
	handlerFn   HandlerFn

	logLevel  logrus.Level
	logFields logrus.Fields
}

type HandlerDefinitionBuilder struct {
	handlerDefinition handlerDefinition
	customLogLevelSet bool
}

func newHandlerDefinitionBuilder() *HandlerDefinitionBuilder {
	return &HandlerDefinitionBuilder{
		customLogLevelSet: false,
	}
}

// LogResultAs adds the arbitrary result information (e.g. `ok` or `failed`) to the log
// line specified for the handler.
func (hdb *HandlerDefinitionBuilder) LogResultAs(result string) *HandlerDefinitionBuilder {
	hdb.handlerDefinition.logArgument = result

	return hdb
}

// WithLogLevel allows to adjust the log level that should be used when logging
// a message for handled case. By default the log level uses two values, depending
// on the status code (one default for "positive" responses, another one for "failure"
// responses).
// When WithLogLevel() is used, the log level for the definition is set explicitly and the
// default values will be not used.
// Usually you will not want to use this method and just relay on the default values! :)
func (hdb *HandlerDefinitionBuilder) WithLogLevel(level logrus.Level) *HandlerDefinitionBuilder {
	hdb.handlerDefinition.logLevel = level
	hdb.customLogLevelSet = true

	return hdb
}

// WithLogFields allows to define additional log fields that should be used when
// logging the information about handled result.
func (hdb *HandlerDefinitionBuilder) WithLogFields(fields logrus.Fields) *HandlerDefinitionBuilder {
	hdb.handlerDefinition.logFields = fields

	return hdb
}

// WithHandlerFn allows to register an arbitrary function that should be
// executed for the specified response case.
func (hdb *HandlerDefinitionBuilder) WithHandlerFn(fn HandlerFn) *HandlerDefinitionBuilder {
	hdb.handlerDefinition.handlerFn = fn

	return hdb
}

// asDefault means that this handler definition will catch all response
// for which we didn't specify a custom handler definition by using the `withCode()`.
func (hdb *HandlerDefinitionBuilder) asDefault() *HandlerDefinitionBuilder {
	return hdb.withCode(defaultHandlerCode)
}

// withCode will make the handler definition to be used only for a specific
// response code.
func (hdb *HandlerDefinitionBuilder) withCode(code int) *HandlerDefinitionBuilder {
	hdb.handlerDefinition.code = code
	hdb.setLogLevelFor(code)

	return hdb
}

func (hdb *HandlerDefinitionBuilder) setLogLevelFor(code int) {
	if hdb.customLogLevelSet {
		return
	}

	hdb.handlerDefinition.logLevel = defaultLogLevel
	if isErrorStatus(code) {
		hdb.handlerDefinition.logLevel = defaultFailureLogLevel
	}
}
