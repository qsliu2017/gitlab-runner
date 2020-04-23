package response_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/network/internal/response"
)

func TestContentTypeHeaderParsingError_Error(t *testing.T) {
	err := response.NewContentTypeHeaderParsingError("test", assert.AnError)
	expected := `parsing "test": assert.AnError general error for testing`
	assert.Equal(t, expected, err.Error())
}

func TestContentTypeHeaderParsingError_Unwrap(t *testing.T) {
	err := response.NewContentTypeHeaderParsingError("test", assert.AnError)
	assert.Equal(t, assert.AnError, err.Unwrap())
}

func TestContentTypeHeaderParsingError_Is(t *testing.T) {
	err := response.NewContentTypeHeaderParsingError("test", assert.AnError)
	assert.True(t, new(response.ContentTypeHeaderParsingError).Is(err))
}

func TestUnexpectedContentTypeError_Error(t *testing.T) {
	err := response.NewUnexpectedContentTypeError("test")
	expected := `server should return application/json, got: test`
	assert.Equal(t, expected, err.Error())
}

func TestUnexpectedContentTypeError_Is(t *testing.T) {
	err := response.NewUnexpectedContentTypeError("test")
	assert.True(t, new(response.UnexpectedContentTypeError).Is(err))
}

func TestNewSimple(t *testing.T) {
	r := response.NewSimple(100, "test")
	assert.Equal(t, 100, r.StatusCode())
	assert.Equal(t, "test", r.Status())
}

func TestNew(t *testing.T) {
	tests := map[string]struct {
		getHTTPResponse func() *http.Response
		assertResponse  func(t *testing.T, result *response.Response)
	}{
		"no *http.Response given": {
			getHTTPResponse: func() *http.Response { return nil },
			assertResponse: func(t *testing.T, result *response.Response) {
				assert.Equal(t, "no status", result.Status())
				assert.Equal(t, -1, result.StatusCode())
				assert.Empty(t, result.Header())
			},
		},
		"*http.Response given": {
			getHTTPResponse: func() *http.Response {
				body := ioutil.NopCloser(new(bytes.Buffer))

				resp := &http.Response{
					StatusCode: 100,
					Status:     "test",
					Body:       body,
				}

				return resp
			},
			assertResponse: func(t *testing.T, result *response.Response) {
				assert.Equal(t, 100, result.StatusCode())
				assert.Equal(t, "test", result.Status())
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			resp := tt.getHTTPResponse()

			r := response.New(resp)
			tt.assertResponse(t, r)
		})
	}
}

func TestResponse_IsApplicationJSON(t *testing.T) {
	newHTTPResponse := func(contentType string) *http.Response {
		r := &http.Response{
			Header: make(http.Header),
			Body:   ioutil.NopCloser(new(bytes.Buffer)),
		}
		r.Header.Add("Content-Type", contentType)

		return r
	}

	tests := map[string]struct {
		httpResponse  *http.Response
		expectedError error
	}{
		"missing response data": {
			httpResponse:  nil,
			expectedError: response.ErrNoHTTPResponse,
		},
		"content-type header parsing error": {
			httpResponse:  newHTTPResponse(""),
			expectedError: new(response.ContentTypeHeaderParsingError),
		},
		"unexpected content-type": {
			httpResponse:  newHTTPResponse("text/plain"),
			expectedError: new(response.UnexpectedContentTypeError),
		},
		"expected content-type": {
			httpResponse: newHTTPResponse("application/json"),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := response.New(tt.httpResponse)

			err := r.IsApplicationJSON()

			if tt.expectedError != nil {
				assert.True(t, errors.Is(err, tt.expectedError))
				return
			}

			assert.NoError(t, err)
		})
	}
}
