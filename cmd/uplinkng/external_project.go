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

	accessDefault, accesses, err := ex.GetAccessInfo(true)
	if err != nil {
		return nil, err
	}
	if accessName != "" {
		accessDefault = accessName
	}

	var access *uplink.Access
	if data, ok := accesses[accessDefault]; ok {
		access, err = uplink.ParseAccess(data)
	} else {
		access, err = uplink.ParseAccess(accessDefault)
		// TODO: if this errors then it's probably a name so don't report an error
		// that says "it failed to parse"
	}
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
