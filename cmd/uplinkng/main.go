// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/zeebo/clingy"

	_ "storj.io/private/process"
	"storj.io/storj/cmd/uplinkng/ulext"
)

func main() {
	ex := newExternal()
	ok, err := clingy.Environment{
		Name:    "uplink",
		Args:    os.Args[1:],
		Dynamic: ex.Dynamic,
		Wrap:    ex.Wrap,
	}.Run(context.Background(), func(cmds clingy.Commands) {
		ex.Setup(cmds) // setup ex first so that stdlib flags can consult config
		newStdlibFlags(flag.CommandLine).Setup(cmds)
		commands(cmds, ex)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	if !ok || err != nil {
		os.Exit(1)
	}
}

func commands(cmds clingy.Commands, ex ulext.External) {
	cmds.Group("access", "Access related commands", func() {
		cmds.New("save", "Save an existing access", newCmdAccessSave(ex))
		cmds.New("create", "Create an access from a setup token", newCmdAccessCreate(ex))
		cmds.New("delete", "Delete an access from local store", newCmdAccessDelete(ex))
		cmds.New("restrict", "Restrict an access", newCmdAccessRestrict(ex))
		cmds.New("list", "List saved accesses", newCmdAccessList(ex))
		cmds.New("use", "Set default access to use", newCmdAccessUse(ex))
		cmds.New("revoke", "Revoke an access", newCmdAccessRevoke(ex))
		cmds.New("inspect", "Inspect allows you to explode a serialized access into its constituent parts", newCmdAccessInspect(ex))
		cmds.New("register", "Register an access grant for use with a hosted S3 compatible gateway and linksharing", newCmdAccessRegister(ex))
	})
	cmds.New("setup", "An alias for access create", newCmdAccessCreate(ex))
	cmds.New("share", "Shares restricted accesses to objects", newCmdShare(ex))
	cmds.New("mb", "Create a new bucket", newCmdMb(ex))
	cmds.New("rb", "Remove a bucket bucket", newCmdRb(ex))
	cmds.New("cp", "Copies files or objects into or out of storj", newCmdCp(ex))
	cmds.New("mv", "Moves files or objects", newCmdMv(ex))
	cmds.New("ls", "Lists buckets, prefixes, or objects", newCmdLs(ex))
	cmds.New("rm", "Remove an object", newCmdRm(ex))
	cmds.Group("meta", "Object metadata related commands", func() {
		cmds.New("get", "Get an object's metadata", newCmdMetaGet(ex))
	})
	cmds.New("version", "Prints version information", newCmdVersion())
}
