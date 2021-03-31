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
	env := clingy.Environment{
		Name: "uplink",
		Args: os.Args[1:],

		Dynamic: gf.Dynamic,
		Wrap:    gf.Wrap,
	}

	ok, err := env.Run(context.Background(), func(c clingy.Commands, f clingy.Flags) {
		// setup the dynamic global flags first so that they may be consulted
		// by the stdlib flags during their definition.
		gf.Setup(f)
		newStdlibFlags(flag.CommandLine).Setup(f)

		c.Group("access", "Access related commands", func() {
			c.New("save", "Save an existing access", new(cmdAccessSave))
			c.New("create", "Create an access from a setup token", new(cmdAccessCreate))
			c.New("delete", "Delete an access from local store", new(cmdAccessDelete))
			c.New("list", "List saved accesses", new(cmdAccessList))
			c.New("use", "Set default access to use", new(cmdAccessUse))
			c.New("revoke", "Revoke an access", new(cmdAccessRevoke))
		})
		c.New("share", "Shares restricted accesses to objects", new(cmdShare))
		c.New("mb", "Create a new bucket", new(cmdMb))
		c.New("rb", "Remove a bucket bucket", new(cmdRb))
		c.New("cp", "Copies files or objects into or out of tardigrade", new(cmdCp))
		c.New("ls", "Lists buckets, prefixes, or objects", new(cmdLs))
		c.New("rm", "Remove an object", new(cmdRm))
		c.Group("meta", "Object metadata related commands", func() {
			c.New("get", "Get an object's metadata", new(cmdMetaGet))
		})
		c.New("version", "Prints version information", new(cmdVersion))
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	if !ok || err != nil {
		os.Exit(1)
	}
}
