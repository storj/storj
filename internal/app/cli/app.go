// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
	"storj.io/storj/pkg/client"
)

// New creates a new storj cli application with the respective commands and metainfo.
func New() *cli.App {
	app := cli.NewApp()
	app.Name = "storj"
	app.Version = "0.0.2"
	app.Usage = "command line interface to the Storj network"
	app.Commands = []cli.Command{
		{
			Name:      "get-info",
			Usage:     "prints bridge api information",
			ArgsUsage: " ", // no args
			Category:  "bridge api information",
			Action: func(c *cli.Context) error {
				getInfo()
				return nil
			},
		},
		{
			Name:      "list-buckets",
			Usage:     "lists the available buckets",
			ArgsUsage: " ", // no args
			Category:  "working with buckets and files",
			Action: func(c *cli.Context) error {
				listBuckets()
				return nil
			},
		},
	}

	cli.AppHelpTemplate = fmt.Sprintf(`%s
ENVIRONMENT VARIABLES:
   STORJ_BRIDGE          the bridge host (e.g. https://api.storj.io)
   STORJ_BRIDGE_USER     bridge username
   STORJ_BRIDGE_PASS     bridge password
   STORJ_ENCRYPTION_KEY  file encryption key

`, cli.AppHelpTemplate)

	return app
}

func getInfo() {
	env := client.NewEnv()
	info, err := client.GetInfo(env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Storj bridge: %s\n\n"+
		"Title:       %s\n"+
		"Description: %s\n"+
		"Version:     %s\n"+
		"Host:        %s\n",
		env.URL, info.Title, info.Description, info.Version, info.Host)
}

func listBuckets() {
	buckets, err := client.GetBuckets(client.NewEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	for _, b := range buckets {
		fmt.Printf("ID: %s\tDecrypted: %t\t\tCreated: %s\tName: %s\n",
			b.ID, b.Decrypted, b.Created, b.Name)
	}
}
