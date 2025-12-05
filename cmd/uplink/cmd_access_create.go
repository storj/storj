// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
)

type cmdAccessCreate struct {
	ex ulext.External
	am accessMaker

	passphraseStdin bool
	satelliteAddr   string
	apiKey          string
	importAs        string
	exportTo        string

	unencryptedObjectKeys *bool
}

func newCmdAccessCreate(ex ulext.External) *cmdAccessCreate {
	return &cmdAccessCreate{ex: ex}
}

func (c *cmdAccessCreate) Setup(params clingy.Parameters) {
	c.passphraseStdin = params.Flag("passphrase-stdin", "If set, the passphrase is read from stdin, and all other values must be provided.", false,
		clingy.Transform(strconv.ParseBool),
		clingy.Boolean,
	).(bool)

	c.satelliteAddr = params.Flag("satellite-address", "Satellite address from satellite UI (prompted if unspecified)", "").(string)
	c.apiKey = params.Flag("api-key", "API key from satellite UI (prompted if unspecified)", "").(string)
	c.importAs = params.Flag("import-as", "Import the access as this name", "").(string)
	c.exportTo = params.Flag("export-to", "Export the access to this file path", "").(string)

	c.unencryptedObjectKeys = params.Flag("unencrypted-object-keys", "If set, the created access grant won't encrypt object keys", nil,
		clingy.Transform(strconv.ParseBool), clingy.Boolean, clingy.Optional,
	).(*bool)

	params.Break()
	c.am.Setup(params, c.ex)
}

func (c *cmdAccessCreate) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if c.satelliteAddr == "" {
		if c.passphraseStdin {
			return errs.New("Must specify the satellite address as a flag when passphrase-stdin is set.")
		}
		c.satelliteAddr, err = c.ex.PromptInput(ctx, "Satellite address:")
		if err != nil {
			return errs.Wrap(err)
		}
	}

	if c.apiKey == "" {
		if c.passphraseStdin {
			return errs.New("Must specify the api key as a flag when passphrase-stdin is set.")
		}
		c.apiKey, err = c.ex.PromptInput(ctx, "API key:")
		if err != nil {
			return errs.Wrap(err)
		}
	}

	unencryptedObjectKeys := false
	if c.unencryptedObjectKeys != nil {
		unencryptedObjectKeys = *c.unencryptedObjectKeys
	} else if !c.passphraseStdin {
		answer, err := c.ex.PromptInput(ctx, "Would you like to disable encryption for object keys (allows lexicographical sorting of objects in listings)? (y/N):")
		if err != nil {
			return errs.Wrap(err)
		}

		answer = strings.ToLower(answer)
		if answer == "y" || answer == "yes" {
			unencryptedObjectKeys = true
		}
	}

	var passphrase string
	if c.passphraseStdin {
		stdinData, err := io.ReadAll(clingy.Stdin(ctx))
		if err != nil {
			return errs.Wrap(err)
		}
		passphrase = strings.TrimRight(string(stdinData), "\r\n")
	} else {
		passphrase, err = c.ex.PromptSecret(ctx, "Passphrase:")
		if err != nil {
			return errs.Wrap(err)
		}
	}
	if passphrase == "" {
		return errs.New("Encryption passphrase must be non-empty")
	}

	access, err := c.ex.RequestAccess(ctx, c.satelliteAddr, c.apiKey, passphrase, unencryptedObjectKeys)
	if err != nil {
		return errs.Wrap(err)
	}

	access, err = c.am.Execute(ctx, c.importAs, access)
	if err != nil {
		return errs.Wrap(err)
	}

	if c.exportTo != "" {
		return c.ex.ExportAccess(ctx, access, c.exportTo)
	}

	if c.importAs != "" {
		return nil
	}

	serialized, err := access.Serialize()
	if err != nil {
		return errs.Wrap(err)
	}

	_, _ = fmt.Fprintln(clingy.Stdout(ctx), serialized)

	return nil
}
