// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// revocations is an implementation of satellite.Revocations
type revocations struct {
	db dbx.Methods
}

// GetByProjectID retrieves list of Revocations for given projectID
func (r *revocations) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([][]byte, error) {
	rows, err := r.db.All_Revocation_By_ApiKey_ProjectId(ctx,
		dbx.ApiKey_ProjectId(projectID[:]))
	if err != nil {
		return nil, err
	}

	out := make([][]byte, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Head)
	}
	return out, nil
}

// Revoked returns true if the provided head has been revoked.
func (r *revocations) Revoked(ctx context.Context, head []byte) (bool, error) {
	return r.db.Has_Revocation_By_Head(ctx,
		dbx.Revocation_Head(head))
}

// Revoke revokes a head.
func (r *revocations) Revoke(ctx context.Context, head []byte) error {
	return r.db.CreateNoReturn_Revocation(ctx,
		dbx.Revocation_Head(head))
}

// Unrevoke unrevokes a head.
func (r *revocations) Unrevoke(ctx context.Context, head []byte) (bool, error) {
	return r.db.Delete_Revocation_By_Head(ctx,
		dbx.Revocation_Head(head))
}
