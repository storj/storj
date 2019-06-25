// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package setup

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

var (
	mon   = monkit.Package()
	Error = errs.Class("uplink setup")
)

// LoadEncryptionCtx loads an EncryptionCtx from the values specified in the encryption config.
func LoadEncryptionCtx(ctx context.Context, cfg uplink.EncryptionConfig) (_ *libuplink.EncryptionCtx, err error) {
	defer mon.Task()(&ctx)(&err)

	if cfg.EncCtxFilepath != "" {
		data, err := ioutil.ReadFile(cfg.EncCtxFilepath)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		return libuplink.ParseEncryptionCtx(strings.TrimSpace(string(data)))
	}

	data := []byte(cfg.EncryptionKey)
	if cfg.KeyFilepath != "" {
		data, err = ioutil.ReadFile(cfg.KeyFilepath)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	key, err := storj.NewKey(data)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return libuplink.NewEncryptionCtxWithDefaultKey(*key), nil
}
