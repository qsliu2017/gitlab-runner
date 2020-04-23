package response

import (
	"net/http"
)

type gitlabAPIErrorBody struct {
	Message string `json:"message"`
}

func addAPIErrorToLogger(logger Logger, r *Response) Logger {
	if !isErrorStatus(r.StatusCode()) {
		return logger
	}

	var apiError gitlabAPIErrorBody

	err := r.DecodeJSONFromBody(&apiError)
	if err != nil {
		return logger
	}

	return logger.WithField("api_error", apiError.Message)
}

func isErrorStatus(statusCode int) bool {
	return statusCode < 0 || statusCode >= http.StatusBadRequest
}
