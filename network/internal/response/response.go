package response

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
)

const (
	applicationJSONContentType = "application/json"
)

var (
	ErrNoHTTPResponse = errors.New("no http.Response provided")
	ErrBodyFlushed    = errors.New("body already flushed")
)

type ContentTypeHeaderParsingError struct {
	innerErr error
	value    string
}

func NewContentTypeHeaderParsingError(value string, innerErr error) *ContentTypeHeaderParsingError {
	return &ContentTypeHeaderParsingError{
		innerErr: innerErr,
		value:    value,
	}
}

func (e *ContentTypeHeaderParsingError) Error() string {
	return fmt.Sprintf("parsing %q: %v", e.value, e.innerErr)
}

func (e *ContentTypeHeaderParsingError) Unwrap() error {
	return e.innerErr
}

func (e *ContentTypeHeaderParsingError) Is(err error) bool {
	_, ok := err.(*ContentTypeHeaderParsingError)

	return ok
}

type UnexpectedContentTypeError struct {
	foundContentType string
}

func NewUnexpectedContentTypeError(contentType string) *UnexpectedContentTypeError {
	return &UnexpectedContentTypeError{
		foundContentType: contentType,
	}
}

func (e *UnexpectedContentTypeError) Error() string {
	return fmt.Sprintf("server should return application/json, got: %v", e.foundContentType)
}

func (e *UnexpectedContentTypeError) Is(err error) bool {
	_, ok := err.(*UnexpectedContentTypeError)

	return ok
}

type Response struct {
	inner *http.Response

	bodyFlushed bool
}

func NewError(err error) *Response {
	return NewSimple(-1, err.Error())
}

func NewSimple(code int, status string) *Response {
	resp := New(&http.Response{
		Status:     status,
		StatusCode: code,
		Body:       ioutil.NopCloser(new(bytes.Buffer)),
		Header:     make(http.Header),
	})

	return resp
}

func New(resp *http.Response) *Response {
	return &Response{
		inner: resp,
	}
}

func (r *Response) discardBody() {
	// We discard the body so don't really care about the errors here
	_ = r.FlushBodyTo(ioutil.Discard)
}

func (r *Response) FlushBodyTo(w io.Writer) error {
	if r.inner == nil {
		return ErrNoHTTPResponse
	}

	if r.bodyFlushed {
		return ErrBodyFlushed
	}

	_, err := io.Copy(w, r.inner.Body)
	if err != nil {
		return fmt.Errorf("copying response body: %w", err)
	}

	err = r.inner.Body.Close()
	if err != nil {
		return fmt.Errorf("closing response body: %w", err)
	}

	r.bodyFlushed = true

	return nil
}

func (r *Response) DecodeJSONFromBody(target interface{}) error {
	if r.inner == nil {
		return ErrNoHTTPResponse
	}

	if r.bodyFlushed {
		return ErrBodyFlushed
	}

	body, err := ioutil.ReadAll(r.inner.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	newBody := make([]byte, len(body))
	copy(newBody, body)

	r.inner.Body = ioutil.NopCloser(bytes.NewBuffer(newBody))

	err = json.Unmarshal(body, target)
	if err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	return nil
}

func (r *Response) Status() string {
	if r.inner == nil {
		return "no status"
	}

	return r.inner.Status
}

func (r *Response) StatusCode() int {
	if r.inner == nil {
		return -1
	}

	return r.inner.StatusCode
}

func (r *Response) Header() http.Header {
	if r.inner == nil {
		return make(http.Header)
	}

	return r.inner.Header
}

func (r *Response) TLS() *tls.ConnectionState {
	if r.inner == nil {
		return nil
	}

	return r.inner.TLS
}

func (r *Response) IsApplicationJSON() error {
	if r.inner == nil {
		return ErrNoHTTPResponse
	}

	contentType := r.inner.Header.Get("Content-Type")
	mimeType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return NewContentTypeHeaderParsingError(contentType, err)
	}

	if mimeType != applicationJSONContentType {
		return NewUnexpectedContentTypeError(contentType)
	}

	return nil
}
