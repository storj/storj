// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/uplink"
	privateAccess "storj.io/uplink/private/access"
)

type projectProvider struct {
	access string

	testProject    *uplink.Project
	testFilesystem ulfs.Filesystem
}

func (pp *projectProvider) Setup(a clingy.Arguments, f clingy.Flags) {
	pp.access = f.New("access", "Which access to use", "").(string)
}

func (pp *projectProvider) SetTestFilesystem(fs ulfs.Filesystem) { pp.testFilesystem = fs }

func (pp *projectProvider) OpenFilesystem(ctx context.Context, options ...projectOption) (ulfs.Filesystem, error) {
	if pp.testFilesystem != nil {
		return pp.testFilesystem, nil
	}

	project, err := pp.OpenProject(ctx, options...)
	if err != nil {
		return nil, err
	}
	return ulfs.NewMixed(ulfs.NewLocal(), ulfs.NewRemote(project)), nil
}

func (pp *projectProvider) OpenProject(ctx context.Context, options ...projectOption) (*uplink.Project, error) {
	if pp.testProject != nil {
		return pp.testProject, nil
	}

	var opts projectOptions
	for _, opt := range options {
		opt.apply(&opts)
	}

	accessDefault, accesses, err := gf.GetAccessInfo()
	if err != nil {
		return nil, err
	}
	if pp.access != "" {
		accessDefault = pp.access
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

	if opts.encryptionBypass {
		if err := privateAccess.EnablePathEncryptionBypass(access); err != nil {
			return nil, err
		}
	}

	return uplink.OpenProject(ctx, access)
}

type projectOptions struct {
	encryptionBypass bool
}

type projectOption struct {
	apply func(*projectOptions)
}

func bypassEncryption(bypass bool) projectOption {
	return projectOption{apply: func(opt *projectOptions) { opt.encryptionBypass = bypass }}
}
