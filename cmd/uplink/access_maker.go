// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/uplink"
)

type accessMaker struct {
	ex ulext.External

	force bool
	use   bool

	perms accessPermissions
}

func (am *accessMaker) Setup(params clingy.Parameters, ex ulext.External) {
	am.ex = ex

	am.force = params.Flag("force", "Force overwrite an existing saved access", false,
		clingy.Short('f'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)

	am.use = params.Flag("use", "Switch the default access to the newly created one", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
}

func (am *accessMaker) Execute(ctx context.Context, name string, access *uplink.Access) (_ *uplink.Access, err error) {
	defer mon.Task()(&ctx)(&err)

	accessInfoFile, err := am.ex.AccessInfoFile()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	defaultName, accesses, err := am.ex.GetAccessInfo(false)
	if err != nil {
		return nil, err
	}

	if name != "" {
		if _, ok := accesses[name]; ok && !am.force {
			return nil, errs.New("Access %q already exists. Overwrite by specifying --force or choose a new name.", name)
		}
	}

	access, err = am.perms.Apply(access)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	accessValue, err := access.Serialize()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if name != "" {
		accesses[name] = accessValue
		if am.use || defaultName == "" {
			defaultName = name
		}

		if err := am.ex.SaveAccessInfo(defaultName, accesses); err != nil {
			return nil, errs.Wrap(err)
		}

		_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Imported access %q to %q\n", name, accessInfoFile)
	}

	return access, nil
}
