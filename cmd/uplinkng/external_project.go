// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/uplink"
	privateAccess "storj.io/uplink/private/access"
)

func (ex *external) OpenFilesystem(ctx context.Context, accessName string, options ...ulext.Option) (ulfs.Filesystem, error) {
	project, err := ex.OpenProject(ctx, accessName, options...)
	if err != nil {
		return nil, err
	}
	return ulfs.NewMixed(ulfs.NewLocal(), ulfs.NewRemote(project)), nil
}

func (ex *external) OpenProject(ctx context.Context, accessName string, options ...ulext.Option) (*uplink.Project, error) {
	opts := ulext.LoadOptions(options...)

	access, err := ex.OpenAccess(accessName)
	if err != nil {
		return nil, err
	}

	if opts.EncryptionBypass {
		if err := privateAccess.EnablePathEncryptionBypass(access); err != nil {
			return nil, err
		}
	}

	return uplink.OpenProject(ctx, access)
}
