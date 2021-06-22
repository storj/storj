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

type accessMaker struct {
	ex ulext.External

	print bool
	save  bool
	name  string
	force bool
	use   bool

	perms accessPermissions
}

func (am *accessMaker) Setup(params clingy.Parameters, ex ulext.External, forceSave bool) {
	am.ex = ex
	am.save = forceSave
	am.print = !forceSave

	if !forceSave {
		am.save = params.Flag("save", "Save the access", true,
			clingy.Transform(strconv.ParseBool),
		).(bool)
	}

	am.name = params.Flag("name", "Name to save newly created access, if --save is true", "").(string)

	am.force = params.Flag("force", "Force overwrite an existing saved access grant", false,
		clingy.Short('f'),
		clingy.Transform(strconv.ParseBool),
	).(bool)

	am.use = params.Flag("use", "Set the saved access to be the one used by default", false,
		clingy.Transform(strconv.ParseBool),
	).(bool)

	if !forceSave {
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

	if am.print {
		fmt.Fprintln(ctx, accessValue)
	}

	if am.save {
		accesses[am.name] = accessValue
		if am.use || defaultName == "" {
			defaultName = am.name
		}

		if err := am.ex.SaveAccessInfo(defaultName, accesses); err != nil {
			return errs.Wrap(err)
		}

		fmt.Fprintf(ctx, "Access %q saved to %q\n", am.name, am.ex.AccessInfoFile())
	}

	return nil
}
