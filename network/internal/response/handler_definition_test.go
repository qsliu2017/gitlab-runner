package response

import (
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testHandlerFnValue = struct{}{}
)

func TestHandlerDefinitionBuilder_SimpleValuesSetting(t *testing.T) {
	testResult := "result"
	testFields := logrus.Fields{"testField": "testValue"}
	testCode := 100

	b := newHandlerDefinitionBuilder()
	b.LogResultAs(testResult).
		WithLogFields(testFields).
		WithHandlerFn(IdentityHandlerFn(&testHandlerFnValue)).
		withCode(testCode)

	hd := b.handlerDefinition
	assert.Equal(t, testCode, hd.code)
	assert.Equal(t, testResult, hd.logArgument)
	require.NotNil(t, hd.handlerFn)
	assert.Equal(t, &testHandlerFnValue, hd.handlerFn(nil))
	assert.Equal(t, testFields, hd.logFields)
}

func TestHandlerDefinitionBuilder_asDefault(t *testing.T) {
	b := newHandlerDefinitionBuilder()
	b.asDefault()

	hd := b.handlerDefinition
	assert.Equal(t, defaultHandlerCode, hd.code)
}

func TestHandlerDefinitionBuilder_setLogLevelFor_usage(t *testing.T) {
	debugLogLevel := logrus.DebugLevel

	tests := map[string]struct {
		statusCode       int
		customLogLevel   *logrus.Level
		expectedLogLevel logrus.Level
	}{
		"successful code defined": {
			statusCode:       http.StatusOK,
			expectedLogLevel: defaultLogLevel,
		},
		"failure code defined": {
			statusCode:       http.StatusBadRequest,
			expectedLogLevel: defaultFailureLogLevel,
		},
		"custom log level defined": {
			statusCode:       http.StatusOK,
			customLogLevel:   &debugLogLevel,
			expectedLogLevel: debugLogLevel,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			b := newHandlerDefinitionBuilder()
			if tt.customLogLevel != nil {
				b.WithLogLevel(*tt.customLogLevel)
			}
			b.withCode(tt.statusCode)

			hd := b.handlerDefinition
			assert.Equal(t, tt.expectedLogLevel, hd.logLevel)
		})
	}
}
