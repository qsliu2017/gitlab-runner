package response

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddAPIErrorToLogger(t *testing.T) {
	apiErrorText := "this is an API error"

	tests := map[string]struct {
		response     *Response
		assertFields func(t *testing.T, fields logrus.Fields)
	}{
		"successful response": {
			response: NewSimple(http.StatusOK, "OK"),
			assertFields: func(t *testing.T, fields logrus.Fields) {
				assert.Empty(t, fields)
			},
		},
		"failure response with body parsing error": {
			response: NewError(assert.AnError),
			assertFields: func(t *testing.T, fields logrus.Fields) {
				assert.Empty(t, fields)
			},
		},
		"failure response with body": {
			response: func() *Response {
				apiErrorBody := `{"message":"` + apiErrorText + `"}`

				r := NewError(assert.AnError)
				r.inner.Body = ioutil.NopCloser(bytes.NewBufferString(apiErrorBody))

				return r
			}(),
			assertFields: func(t *testing.T, fields logrus.Fields) {
				require.Contains(t, fields, "api_error")
				assert.Equal(t, apiErrorText, fields["api_error"])
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			logger, hooks := test.NewNullLogger()

			log := addAPIErrorToLogger(logger, tt.response)
			require.NotNil(t, log)

			log.Info("test")

			entry := hooks.LastEntry()
			require.NotNil(t, entry)

			tt.assertFields(t, entry.Data)
		})
	}
}

func TestIsErrorStatus(t *testing.T) {
	tests := map[string]struct {
		statusCode     int
		expectedResult bool
	}{
		"custom error response code": {
			statusCode:     defaultHandlerCode,
			expectedResult: true,
		},
		"successful response code": {
			statusCode:     http.StatusOK,
			expectedResult: false,
		},
		"failure response code": {
			statusCode:     http.StatusBadRequest,
			expectedResult: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expectedResult, isErrorStatus(tt.statusCode))
		})
	}
}
