// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"context"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/uplink"
)

type external struct {
	ulext.External

	fs      ulfs.Filesystem
	project *uplink.Project
}

func newExternal(fs ulfs.Filesystem, project *uplink.Project) *external {
	return &external{
		fs:      fs,
		project: project,
	}
}

func (ex *external) OpenFilesystem(ctx context.Context, access string, options ...ulext.Option) (ulfs.Filesystem, error) {
	return ex.fs, nil
}

func (ex *external) OpenProject(ctx context.Context, access string, options ...ulext.Option) (*uplink.Project, error) {
	return ex.project, nil
}
