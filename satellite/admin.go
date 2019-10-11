// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"bytes"
	"context"
	"encoding/hex"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/satellite/console"
)

// AdminConfig represents configuration for admin-bypass-type operations
type AdminConfig struct {
	AdminHead   string `help:"the macaroon head to expect for admin keys. In hex" default:""`
	AdminSecret string `help:"the macaroon secret to expect for admin keys. In hex" default:""`
}

type adminKeyWrapper struct {
	console.APIKeys
	head, secret []byte
}

func adminKeys(config AdminConfig, db console.APIKeys) (console.APIKeys, error) {
	var head, secret []byte
	var err error
	if config.AdminHead != "" && config.AdminSecret != "" {
		head, err = hex.DecodeString(config.AdminHead)
		if err != nil {
			return nil, errs.New("failed to decode admin-head as hex: %v", err)
		}
		secret, err = hex.DecodeString(config.AdminSecret)
		if err != nil {
			return nil, errs.New("failed to decode admin-secret as hex: %v", err)
		}
	}
	return &adminKeyWrapper{APIKeys: db, head: head, secret: secret}, nil
}

func (w *adminKeyWrapper) GetByKey(ctx context.Context, key *macaroon.APIKey) (_ *console.APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	// we don't need constant time comparison here. the head isn't secret.
	if len(w.head) > 0 && bytes.Equal(w.head, key.Head()) {
		projectID, err := key.GetProjectID(ctx)
		if err != nil {
			return nil, errs.New("invalid admin key: %v", err)
		}
		if len(projectID) == 0 {
			return nil, errs.New("no project id provided for admin key")
		}
		return &console.APIKeyInfo{
			ProjectID: *projectID,
			Name:      "[Administrator Key]",
			Secret:    w.secret,
		}, nil
	}

	return w.APIKeys.GetByKey(ctx, key)
}
