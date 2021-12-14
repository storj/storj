// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/uplink"
)

type amSaveKind int

const (
	amSaveDefaultFalse amSaveKind = iota
	amSaveDefaultTrue
	amSaveForced
)

type accessMaker struct {
	ex ulext.External

	save  bool
	name  string
	force bool
	use   bool

	perms accessPermissions
}

func (am *accessMaker) Setup(params clingy.Parameters, ex ulext.External, saveKind amSaveKind) {
	am.ex = ex
	am.save = saveKind == amSaveForced

	if saveKind != amSaveForced {
		am.save = params.Flag("save", "Save the access", saveKind == amSaveDefaultTrue,
			clingy.Transform(strconv.ParseBool), clingy.Boolean,
		).(bool)

		am.name = params.Flag("name", "Name to save the access value under, if --save is true", "").(string)
	} else {
		am.name = params.Flag("name", "Name to save the access value under", "").(string)
	}

	am.force = params.Flag("force", "Force overwrite an existing saved access", false,
		clingy.Short('f'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)

	am.use = params.Flag("use", "Switch the access to be the default", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)

	if saveKind != amSaveForced {
		params.Break()
		am.perms.Setup(params)
	}
}

func (am *accessMaker) Execute(ctx clingy.Context, access *uplink.Access) (err error) {
	defaultName, accesses, err := am.ex.GetAccessInfo(false)
	if err != nil {
		return err
	}

	if am.save {
		// pick a default name for the access if we're saving and there are
		// no saved accesses. otherwise, prompt.
		if am.name == "" && len(accesses) == 0 {
			am.name = "default"
		}

		if am.name == "" {
			am.name, err = am.ex.PromptInput(ctx, "Name:")
			if err != nil {
				return errs.Wrap(err)
			}
		}

		if _, ok := accesses[am.name]; ok && !am.force {
			return errs.New("Access %q already exists. Overwrite by specifying --force or choose a new name with --name", am.name)
		}
	}

	access, err = am.perms.Apply(access)
	if err != nil {
		return errs.Wrap(err)
	}

	accessValue, err := access.Serialize()
	if err != nil {
		return errs.Wrap(err)
	}

	if am.save {
		accesses[am.name] = accessValue
		if am.use || defaultName == "" {
			defaultName = am.name
		}

		if err := am.ex.SaveAccessInfo(defaultName, accesses); err != nil {
			return errs.Wrap(err)
		}

		fmt.Fprintf(ctx, "Saved access %q to %q\n", am.name, am.ex.AccessInfoFile())
	} else {
		fmt.Fprintln(ctx, accessValue)
	}

	return nil
}
