package gitlab_release

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gitlab.com/gitlab-org/release-cli/gitlab"
)

type Releaser struct {
	gitlabURL string
	jobToken  string
	projectID string
}

func New(gitlabURL string, jobToken string, projectID int) *Releaser {
	return &Releaser{
		gitlabURL: gitlabURL,
		jobToken:  jobToken,
		projectID: strconv.Itoa(projectID),
	}
}

func (r *Releaser) Do(ctx context.Context, release *gitlab.CreateReleaseRequest) (*gitlab.CreateReleaseResponse, error) {
	g, err := gitlab.New(r.gitlabURL, r.jobToken, "", r.projectID, http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("creating gitlab release client: %w", err)
	}

	if release.ReleasedAt == nil {
		now := time.Now()
		release.ReleasedAt = &now
	}

	for _, link := range release.Assets.Links {
		fmt.Printf("%#+v\n", link)
	}

	response, err := g.CreateRelease(ctx, release)
	if err != nil {
		return nil, fmt.Errorf("publishing release at %q, project %q: %w", r.gitlabURL, r.projectID, err)
	}

	return response, nil
}
