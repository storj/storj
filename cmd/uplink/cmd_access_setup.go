// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/uplink"
)

type cmdAccessSetup struct {
	ex ulext.External
	am accessMaker

	authService string
}

func newCmdAccessSetup(ex ulext.External) *cmdAccessSetup {
	return &cmdAccessSetup{
		ex: ex,
		am: accessMaker{
			ex:  ex,
			use: true,
		},
	}
}

func (c *cmdAccessSetup) Setup(params clingy.Parameters) {
	c.authService = params.Flag("auth-service", "If generating backwards-compatible S3 Gateway credentials, use this auth service", "https://auth.storjshare.io").(string)

	c.am.Setup(params, c.ex)
}

func (c *cmdAccessSetup) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	name, err := c.ex.PromptInput(ctx, "Enter name to import as [default: main]:")
	if err != nil {
		return errs.Wrap(err)
	}
	if name == "" {
		name = "main"
	}

	keyOrGrant, err := c.ex.PromptInput(ctx, "Enter API key or Access grant:")
	if err != nil {
		return errs.Wrap(err)
	}
	if keyOrGrant == "" {
		return errs.New("API key cannot be empty.")
	}

	access, err := uplink.ParseAccess(keyOrGrant)
	if err == nil {
		_, err := c.am.Execute(ctx, name, access)
		if err != nil {
			return errs.Wrap(err)
		}
	} else {
		satelliteAddr, err := c.ex.PromptInput(ctx, "Satellite address:")
		if err != nil {
			return errs.Wrap(err)
		}
		if satelliteAddr == "" {
			return errs.New("Satellite address cannot be empty.")
		}

		passphrase, err := c.ex.PromptSecret(ctx, "Passphrase:")
		if err != nil {
			return errs.Wrap(err)
		}
		if passphrase == "" {
			return errs.New("Encryption passphrase cannot be empty.")
		}

		unencryptedObjectKeys := false
		answer, err := c.ex.PromptInput(ctx, "Would you like to disable encryption for object keys (allows lexicographical sorting of objects in listings)? (y/N):")
		if err != nil {
			return errs.Wrap(err)
		}

		answer = strings.ToLower(answer)
		if answer == "y" || answer == "yes" {
			unencryptedObjectKeys = true
		}

		access, err = c.ex.RequestAccess(ctx, satelliteAddr, keyOrGrant, passphrase, unencryptedObjectKeys)
		if err != nil {
			return errs.Wrap(err)
		}

		_, err = c.am.Execute(ctx, name, access)
		if err != nil {
			return errs.Wrap(err)
		}
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Switched default access to %q\n", name)

	answer, err := c.ex.PromptInput(ctx, "Would you like S3 backwards-compatible Gateway credentials? (y/N):")
	if err != nil {
		return errs.Wrap(err)
	}
	answer = strings.ToLower(answer)

	if answer != "y" && answer != "yes" {
		return nil
	}

	info, err := c.ex.GetEdgeUrlOverrides(ctx, access)
	if err != nil {
		return errs.New("could not get project info: %w", err)
	}

	authService := c.authService

	if info.AuthService != "" {
		authService = info.AuthService
	}

	credentials, err := RegisterAccess(ctx, access, authService, false, "")
	if err != nil {
		return errs.Wrap(err)
	}
	return errs.Wrap(DisplayGatewayCredentials(ctx, *credentials, "", ""))
}
