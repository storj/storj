// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeapiversion"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type nodeAPIVersionDB struct {
	db *satelliteDB
}

// UpdateVersionAtLeast sets the node version to be at least the passed in version.
// Any existing entry for the node will never have the version decreased.
func (db *nodeAPIVersionDB) UpdateVersionAtLeast(ctx context.Context, id storj.NodeID, version nodeapiversion.Version) (err error) {
	defer mon.Task()(&ctx)(&err)
	// try to create a row at the version
	err = db.db.ReplaceNoReturn_NodeApiVersion(ctx,
		dbx.NodeApiVersion_Id(id.Bytes()),
		dbx.NodeApiVersion_ApiVersion(int(version)))
	if dbx.IsConstraintError(err) {
		// if it's a constraint error, the row already exists, so try to update it
		// if the existing value is smaller.
		err = db.db.UpdateNoReturn_NodeApiVersion_By_Id_And_ApiVersion_Less(ctx,
			dbx.NodeApiVersion_Id(id.Bytes()),
			dbx.NodeApiVersion_ApiVersion(int(version)),
			dbx.NodeApiVersion_Update_Fields{
				ApiVersion: dbx.NodeApiVersion_ApiVersion(int(version)),
			})
	}
	return errs.Wrap(err)
}

// VersionAtLeast returns true iff the recorded node version is greater than or equal
// to the passed in version. VersionAtLeast always returns true if the passed in version
// is HasAnything.
func (db *nodeAPIVersionDB) VersionAtLeast(ctx context.Context, id storj.NodeID, version nodeapiversion.Version) (bool, error) {
	if version == nodeapiversion.HasAnything {
		return true, nil
	}
	return db.db.Has_NodeApiVersion_By_Id_And_ApiVersion_GreaterOrEqual(ctx,
		dbx.NodeApiVersion_Id(id.Bytes()),
		dbx.NodeApiVersion_ApiVersion(int(version)))
}
