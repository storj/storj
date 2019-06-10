// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package valueattribution

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/console"
)

// Error messages
const (
	InternalErrMsg         = "It looks like we had a problem on our end. Please try again"
	NoConnectorIDSetErrMsg = "No ConnectorID set"
)

// connectorID is a context value key type
type connectorID string

// connectorKey is context key for connector ID
var connectorKey connectorID = "CONNECTORID"

// ConnectorKeyInfo describing connector/partner key info in the database
type ConnectorKeyInfo struct {
	PartnerID []byte
	BucketID  []byte
	FullName  string
	ShortName string
	Email     string
	Status    console.UserStatus
	CreatedAt time.Time
}

// DB implements the database for value attribution table
type DB interface {
	// GetByProjectID retrieves list of ConnectorKey for given projectID
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*pb.ConnectorKeyInfo, error)
	// Create creates and stores new ConnectorKeyInfo
	Create(ctx context.Context, info *pb.ConnectorKeyInfo) (*pb.ConnectorKeyInfo, error)
	// Delete deletes ConnectorKeyInfo from store
	Delete(ctx context.Context, id uuid.UUID) error
}

// WithConnectorKey creates new context with partner connector ID
func WithConnectorKey(ctx context.Context, auth ConnectorKeyInfo) context.Context {
	return context.WithValue(ctx, connectorKey, auth)
}

// GetConnectorKeyInfo gets partner's connector ID
func GetConnectorKeyInfo(ctx context.Context) (ConnectorKeyInfo, error) {
	value := ctx.Value(connectorKey)

	if auth, ok := value.(ConnectorKeyInfo); ok {
		return auth, nil
	}

	if _, ok := value.(error); ok {
		return ConnectorKeyInfo{}, errs.New(InternalErrMsg)
	}

	return ConnectorKeyInfo{}, errs.New(NoConnectorIDSetErrMsg)
}
