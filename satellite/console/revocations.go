package console

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Revocations is the interface for working with a set of revocations.
type Revocations interface {
	// GetByProjectID retrieves list of Revocations for given projectID
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([][]byte, error)
	// Revoked returns true if the provided head has been revoked.
	Revoked(ctx context.Context, head []byte) (bool, error)
	// Revoke revokes a head.
	Revoke(ctx context.Context, head []byte) error
	// Unrevoke unrevokes a head. Returns true if a head matched.
	Unrevoke(ctx context.Context, head []byte) (bool, error)
}
