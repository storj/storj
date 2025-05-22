// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/encryption"
	"storj.io/common/grant"
	"storj.io/common/paths"
	"storj.io/storj/cmd/uplink/ulext"
)

type cmdDebugDecrypPath struct {
	ex ulext.External

	access        string
	bucket        string
	encryptedPath string
}

func newCmdDebugDecryptPath(ex ulext.External) *cmdDebugDecrypPath {
	return &cmdDebugDecrypPath{
		ex: ex,
	}
}

func (c *cmdDebugDecrypPath) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)

	c.bucket = params.Arg("bucket", "Bucket which contains object").(string)
	c.encryptedPath = params.Arg("encrypted-path", "Path to decrypt").(string)
}

func (c *cmdDebugDecrypPath) Execute(ctx context.Context) error {
	access, err := c.ex.OpenAccess(c.access)
	if err != nil {
		return err
	}

	serializedAccess, err := access.Serialize()
	if err != nil {
		return errs.New("could not serialize access: %+v", err)
	}

	grantAccess, err := grant.ParseAccess(serializedAccess)
	if err != nil {
		return errs.New("could not parse access: %+v", err)
	}

	pathBytes, err := hex.DecodeString(c.encryptedPath)
	if err != nil {
		return errs.New("could not parse object key: %+v", err)
	}

	encPath, err := encryption.DecryptPath(c.bucket, paths.NewEncrypted(string(pathBytes)), grantAccess.EncAccess.Store.GetDefaultPathCipher(), grantAccess.EncAccess.Store)
	if err != nil {
		return errs.New("could not decrypt object key: %+v", err)
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Path: %q\n", encPath.Raw())
	return nil
}
