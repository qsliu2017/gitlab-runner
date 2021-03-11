package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Assets describes the assets as Links associated to a release.
type Assets struct {
	Links []*Link `json:"links"`
}

// Link describes the Link request/response body.
type Link struct {
	ID       int64  `json:"id,omitempty"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	External bool   `json:"external,omitempty"`
	LinkType string `json:"link_type,omitempty"`
	Filepath string `json:"filepath,omitempty"`
}

// Milestone response body when creating a release. Only uses a subset of all the fields.
// The full documentation can be found at https://docs.gitlab.com/ee/api/releases/index.html#create-a-release
type Milestone struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// CreateReleaseRequest body.
// The full documentation can be found at https://docs.gitlab.com/ee/api/releases/index.html#create-a-release
type CreateReleaseRequest struct {
	ID          string     `json:"id"`
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	TagName     string     `json:"tag_name"`
	Ref         string     `json:"ref,omitempty"`
	Assets      *Assets    `json:"assets,omitempty"`
	Milestones  []string   `json:"milestones,omitempty"`
	ReleasedAt  *time.Time `json:"released_at,omitempty"`
}

// CreateReleaseResponse body.
// The full documentation can be found at https://docs.gitlab.com/ee/api/releases/index.html#create-a-release
type CreateReleaseResponse struct {
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	DescriptionHTML string       `json:"description_html"`
	TagName         string       `json:"tag_name"`
	CreatedAt       time.Time    `json:"created_at"`
	ReleasedAt      time.Time    `json:"released_at"`
	Assets          *Assets      `json:"assets,omitempty"`
	Milestones      []*Milestone `json:"milestones,omitempty"`
}

// CreateRelease will try to create a release via GitLab's Releases API
func (gc *Client) CreateRelease(ctx context.Context, createReleaseReq *CreateReleaseRequest) (*CreateReleaseResponse, error) {
	body, err := json.Marshal(createReleaseReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := gc.request(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/releases", gc.projectID), bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	res, err := gc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}

	defer checkClosed(res.Body)

	if res.StatusCode >= http.StatusBadRequest {
		errResponse := ErrorResponse{
			statusCode: res.StatusCode,
		}

		err := json.NewDecoder(res.Body).Decode(&errResponse)
		if err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}

		return nil, &errResponse
	}

	var response CreateReleaseResponse

	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
