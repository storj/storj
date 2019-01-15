package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Buckets interface
type Buckets interface {
	ListBuckets(ctx context.Context, projectID uuid.UUID) ([]Bucket, error)
	GetBucket(ctx context.Context, name string) (*Bucket, error)
	AttachBucket(ctx context.Context, name string, projectID uuid.UUID) (*Bucket, error)
	DeattachBucket(ctx context.Context, name string) error
}

// Bucket type
type Bucket struct {
	ID uuid.UUID

	Name      string
	ProjectID uuid.UUID

	CreatedAt time.Time
}
