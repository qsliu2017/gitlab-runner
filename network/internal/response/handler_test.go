package response_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/network/internal/response"
)

var (
	newOKResponse       = response.NewSimple(http.StatusOK, "OK")
	newNotFoundResponse = response.NewSimple(http.StatusNotFound, "Not found")
)

func TestHandler_Log(t *testing.T) {
	resp := response.NewSimple(http.StatusOK, "OK")
	resp.Header().Set(response.CorrelationIDHeader, "request-id")

	tests := map[string]struct {
		response     *response.Response
		assertOutput func(t *testing.T, output string)
	}{
		"response is nil": {
			response: nil,
			assertOutput: func(t *testing.T, output string) {
				assert.NotContains(t, output, "status_text=OK")
				assert.NotContains(t, output, "status=200")
			},
		},
		"response is not nil": {
			response: resp,
			assertOutput: func(t *testing.T, output string) {
				assert.Contains(t, output, "status_text=OK")
				assert.Contains(t, output, "status=200")
				assert.Contains(t, output, "correlation_id=request-id")
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			l, hooks := test.NewNullLogger()
			l.SetLevel(logrus.DebugLevel)

			rh := response.NewHandler(l, "log line")
			defer rh.Flush()

			rh.SetResponse(tt.response)
			rh.Log(logrus.WarnLevel, "log argument")

			output, err := hooks.LastEntry().String()
			require.NoError(t, err)

			assert.Contains(t, output, `msg="log line log argument"`)
			assert.Contains(t, output, "level=warning")
			tt.assertOutput(t, output)
		})
	}
}

func TestHandler_Handle(t *testing.T) {
	tests := map[string]struct {
		response        *response.Response
		registerHandler func(rh *response.Handler)
		assertLog       func(t *testing.T, output string)
		expectedResult  interface{}
	}{
		"successful response with handler function defined": {
			response: newOKResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusOK).
					LogResultAs("ok").
					WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
						return true
					})
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line ok"`)
				assert.Contains(t, output, "status_text=OK")
				assert.Contains(t, output, "status=200")
				assert.Contains(t, output, "level=info")
			},
			expectedResult: true,
		},
		"failure response with handler function defined": {
			response: newNotFoundResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusNotFound).
					LogResultAs("not-found").
					WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
						return "true"
					})
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line not-found"`)
				assert.Contains(t, output, `status_text="Not found"`)
				assert.Contains(t, output, "status=404")
				assert.Contains(t, output, "level=error")
			},
			expectedResult: "true",
		},
		"successful response with handler function not defined": {
			response: newOKResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusOK).
					LogResultAs("ok").
					WithHandlerFn(nil)
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line ok"`)
				assert.Contains(t, output, "status_text=OK")
				assert.Contains(t, output, "status=200")
				assert.Contains(t, output, "level=info")
			},
			expectedResult: nil,
		},
		"failure response with handler function not defined": {
			response: newNotFoundResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusNotFound).
					LogResultAs("not-found").
					WithHandlerFn(nil)
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line not-found"`)
				assert.Contains(t, output, `status_text="Not found"`)
				assert.Contains(t, output, "status=404")
				assert.Contains(t, output, "level=error")
			},
			expectedResult: nil,
		},
		"successful response with handler for the code not defined and default is defined": {
			response: newOKResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusNotFound).
					LogResultAs("not-found")
				rh.InDefaultCase().
					LogResultAs("default")
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line default"`)
				assert.Contains(t, output, "status_text=OK")
				assert.Contains(t, output, "status=200")
				assert.Contains(t, output, "level=error")
			},
			expectedResult: nil,
		},
		"failure response with handler for the code not defined and default is defined": {
			response: newNotFoundResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusOK).
					LogResultAs("ok")
				rh.InDefaultCase().
					LogResultAs("default")
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line default"`)
				assert.Contains(t, output, `status_text="Not found"`)
				assert.Contains(t, output, "status=404")
				assert.Contains(t, output, "level=error")
			},
			expectedResult: nil,
		},
		"response is nil": {
			response: nil,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusNotFound).
					LogResultAs("not-found")
				rh.InDefaultCase().
					LogResultAs("default")
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line default"`)
				assert.NotContains(t, output, `status_text=`)
				assert.NotContains(t, output, "status=")
			},
			expectedResult: nil,
		},
		"failure response with API error JSON message": {
			response: func() *response.Response {
				resp := &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "Not found",
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"message":"test error message"}`)),
				}

				return response.New(resp)
			}(),
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusNotFound).
					LogResultAs("not-found")
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line not-found"`)
				assert.Contains(t, output, `status_text="Not found"`)
				assert.Contains(t, output, "status=404")
				assert.Contains(t, output, "level=error")
				assert.Contains(t, output, `api_error="test error message"`)
			},
			expectedResult: nil,
		},
		"successful response with changed log level": {
			response: newOKResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusOK).
					LogResultAs("ok").
					WithLogLevel(logrus.DebugLevel).
					WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
						return true
					})
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line ok"`)
				assert.Contains(t, output, "status_text=OK")
				assert.Contains(t, output, "status=200")
				assert.Contains(t, output, "level=debug")
			},
			expectedResult: true,
		},
		"failure response with changed log level": {
			response: newNotFoundResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusNotFound).
					LogResultAs("not-found").
					WithLogLevel(logrus.WarnLevel).
					WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
						return "true"
					})
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line not-found"`)
				assert.Contains(t, output, `status_text="Not found"`)
				assert.Contains(t, output, "status=404")
				assert.Contains(t, output, "level=warning")
			},
			expectedResult: "true",
		},
		"custom log fields set": {
			response: newNotFoundResponse,
			registerHandler: func(rh *response.Handler) {
				rh.WhenCodeIs(http.StatusNotFound).
					LogResultAs("not-found").
					WithLogFields(logrus.Fields{"test": 123}).
					WithHandlerFn(func(_ logrus.FieldLogger) interface{} {
						return 1234
					})
			},
			assertLog: func(t *testing.T, output string) {
				assert.Contains(t, output, `msg="log line not-found"`)
				assert.Contains(t, output, `status_text="Not found"`)
				assert.Contains(t, output, "status=404")
				assert.Contains(t, output, "level=error")
				assert.Contains(t, output, "test=123")
			},
			expectedResult: 1234,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			l, hooks := test.NewNullLogger()
			l.SetLevel(logrus.DebugLevel)

			rh := response.NewHandler(l, "log line")
			defer rh.Flush()

			tt.registerHandler(rh)

			rh.SetResponse(tt.response)

			result := rh.Handle()
			assert.Equal(t, tt.expectedResult, result)

			output, err := hooks.LastEntry().String()
			require.NoError(t, err)
			tt.assertLog(t, output)
		})
	}
}

func TestHandler_AddLogFields(t *testing.T) {
	l, hooks := test.NewNullLogger()

	rh := response.NewHandler(l.WithField("field_1", 1), "log line")
	defer rh.Flush()

	rh.AddLogFields(logrus.Fields{"field_2": 2})

	rh.Log(logrus.InfoLevel, "test")

	output, err := hooks.LastEntry().String()
	require.NoError(t, err)

	assert.Contains(t, output, "log line test")
	assert.Contains(t, output, "field_1=1")
	assert.Contains(t, output, "field_2=2")
}

func TestHandler_AddLogError(t *testing.T) {
	l, hooks := test.NewNullLogger()

	rh := response.NewHandler(l.WithField("field_1", 1), "log line")
	defer rh.Flush()

	rh.AddLogError(errors.New("test error"))

	rh.Log(logrus.InfoLevel, "test")

	output, err := hooks.LastEntry().String()
	require.NoError(t, err)

	assert.Contains(t, output, "log line test")
	assert.Contains(t, output, "field_1=1")
	assert.Contains(t, output, `error="test error"`)
}
