// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"context"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

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

func (ex *external) OpenAccess(accessName string) (access *uplink.Access, err error) {
	return nil, errs.New("not implemented")
}

func (ex *external) GetAccessInfo(required bool) (string, map[string]string, error) {
	return "", nil, errs.New("not implemented")
}

func (ex *external) SaveAccessInfo(accessDefault string, accesses map[string]string) error {
	return errs.New("not implemented")
}

func (ex *external) PromptInput(ctx clingy.Context, prompt string) (input string, err error) {
	return "", errs.New("not implemented")
}

func (ex *external) PromptSecret(ctx clingy.Context, prompt string) (secret string, err error) {
	return "", errs.New("not implemented")
}
