package gitlab

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	log "github.com/sirupsen/logrus"
)

const apiBaseURL = "/api/v4/"

var errMissingToken = errors.New("access token not provided")

// ErrorResponse expected from the API
type ErrorResponse struct {
	statusCode int
	Message    string `json:"message,omitempty"`
	// Err will only be populated if the Releases API returns an unexpected error and is not contained in Message
	Err string `json:"error,omitempty"`
}

// Error implements the error interface. Wraps an error message from the API into an error type
func (er *ErrorResponse) Error() string {
	err := fmt.Sprintf("API Error Response status_code: %d message: %s", er.statusCode, er.Message)

	if er.Err != "" {
		return fmt.Sprintf("%s error: %s", err, er.Err)
	}

	return err
}

// HTTPClient is an interface that describes the available actions of the client.
// Use http.Client during runtime.
// See mock_httpClient_test.go for a testing implementation
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is used to send requests to the GitLab API. Normally created with the `New` function
type Client struct {
	baseURL      string
	jobToken     string
	privateToken string // used outside of CI
	projectID    string
	httpClient   HTTPClient
}

// New creates a new GitLab Client
func New(serverURL, jobToken, privateToken, projectID string, httpClient HTTPClient) (*Client, error) {
	if jobToken == "" && privateToken == "" {
		return nil, errMissingToken
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %w", err)
	}

	u.Path = path.Join(u.Path, apiBaseURL)

	return &Client{
		baseURL:      u.String(),
		jobToken:     jobToken,
		privateToken: privateToken,
		projectID:    projectID,
		httpClient:   httpClient,
	}, nil
}

// request creates a new request and attaches
func (gc *Client) request(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, gc.baseURL+url, body)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	// if PRIVATE-TOKEN takes precedence over JOB-TOKEN
	if gc.privateToken != "" {
		req.Header.Set("PRIVATE-TOKEN", gc.privateToken)
	} else {
		req.Header.Set("JOB-TOKEN", gc.jobToken)
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func checkClosed(closer io.Closer) {
	if err := closer.Close(); err != nil {
		log.WithError(err).Warn("failed to close")
	}
}
