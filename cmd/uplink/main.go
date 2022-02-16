// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/zeebo/clingy"

	_ "storj.io/common/rpc/quic" // include quic connector
	"storj.io/storj/cmd/uplink/ulext"
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
		cmds.New("create", "Create an access from the satellite UI", newCmdAccessCreate(ex))
		cmds.New("export", "Export an access to a file", newCmdAccessExport(ex))
		cmds.New("import", "Import an existing access", newCmdAccessImport(ex))
		cmds.New("inspect", "Inspect shows verbose details about an access", newCmdAccessInspect(ex))
		cmds.New("list", "List saved accesses", newCmdAccessList(ex))
		cmds.New("register", "Register an access grant for use with a hosted S3 compatible gateway and linksharing", newCmdAccessRegister(ex))
		cmds.New("remove", "Removes an access from local store", newCmdAccessRemove(ex))
		cmds.New("restrict", "Restrict an access", newCmdAccessRestrict(ex))
		cmds.New("revoke", "Revoke an access", newCmdAccessRevoke(ex))
		cmds.New("setup", "Wizard for setting up uplink from satellite UI", newCmdAccessSetup(ex))
		cmds.New("use", "Set default access to use", newCmdAccessUse(ex))
	})
	cmds.New("setup", "Wizard for setting up uplink from satellite UI", newCmdAccessSetup(ex))
	cmds.New("mb", "Create a new bucket", newCmdMb(ex))
	cmds.New("rb", "Remove a bucket bucket", newCmdRb(ex))
	cmds.New("cp", "Copies files or objects into or out of storj", newCmdCp(ex))
	cmds.New("mv", "Moves files or objects", newCmdMv(ex))
	cmds.New("ls", "Lists buckets, prefixes, or objects", newCmdLs(ex))
	cmds.New("rm", "Remove an object", newCmdRm(ex))
	cmds.Group("meta", "Object metadata related commands", func() {
		cmds.New("get", "Get an object's metadata", newCmdMetaGet(ex))
	})
	cmds.New("share", "Shares restricted accesses to objects", newCmdShare(ex))
	cmds.New("version", "Prints version information", newCmdVersion())
}
