package referees

import (
	"bytes"
	"context"
	"time"
)

type Referee interface {
	Prepare(executor interface{}) bool
	Execute(
		ctx context.Context,
		startTime time.Time,
		endTime time.Time,
	) (*bytes.Reader, error)
	ArtifactBaseName() string
	ArtifactType() string
	ArtifactFormat() string
}
