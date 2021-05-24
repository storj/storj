// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/zeebo/clingy"
)

var gf = newGlobalFlags()

func main() {
	ok, err := clingy.Environment{
		Name: "uplink",
		Args: os.Args[1:],

		Dynamic: gf.Dynamic,
		Wrap:    gf.Wrap,
	}.Run(context.Background(), func(cmds clingy.Commands) {
		// setup the dynamic global flags first so that they may be consulted
		// by the stdlib flags during their definition.
		gf.Setup(cmds)
		newStdlibFlags(flag.CommandLine).Setup(cmds)

		commands(cmds)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	if !ok || err != nil {
		os.Exit(1)
	}
}

func commands(cmds clingy.Commands) {
	cmds.Group("access", "Access related commands", func() {
		cmds.New("save", "Save an existing access", new(cmdAccessSave))
		cmds.New("create", "Create an access from a setup token", new(cmdAccessCreate))
		cmds.New("delete", "Delete an access from local store", new(cmdAccessDelete))
		cmds.New("list", "List saved accesses", new(cmdAccessList))
		cmds.New("use", "Set default access to use", new(cmdAccessUse))
		cmds.New("revoke", "Revoke an access", new(cmdAccessRevoke))
	})
	cmds.New("share", "Shares restricted accesses to objects", new(cmdShare))
	cmds.New("mb", "Create a new bucket", new(cmdMb))
	cmds.New("rb", "Remove a bucket bucket", new(cmdRb))
	cmds.New("cp", "Copies files or objects into or out of tardigrade", new(cmdCp))
	cmds.New("ls", "Lists buckets, prefixes, or objects", new(cmdLs))
	cmds.New("rm", "Remove an object", new(cmdRm))
	cmds.Group("meta", "Object metadata related commands", func() {
		cmds.New("get", "Get an object's metadata", new(cmdMetaGet))
	})
	cmds.New("version", "Prints version information", new(cmdVersion))
}
