package vault

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/vault/api"
)

func TrimSlashes(path string) string {
	return strings.Trim(path, "/")
}

type errUnwrappedAPIResponse struct {
	statusCode int
	apiErrors  string
}

func newErrUnwrappedAPIResponse(statusCode int, errors []string) *errUnwrappedAPIResponse {
	return &errUnwrappedAPIResponse{
		statusCode: statusCode,
		apiErrors:  strings.Join(errors, ", "),
	}
}

func (e *errUnwrappedAPIResponse) Error() string {
	return fmt.Sprintf("api error: status code %d: %s", e.statusCode, e.apiErrors)
}

func (e *errUnwrappedAPIResponse) Is(err error) bool {
	_, ok := err.(*errUnwrappedAPIResponse)

	return ok
}

func unwrapAPIResponseError(err error) error {
	if err == nil {
		return nil
	}

	apiErr := new(api.ResponseError)
	if !errors.As(err, &apiErr) {
		return err
	}

	return newErrUnwrappedAPIResponse(apiErr.StatusCode, apiErr.Errors)
}
