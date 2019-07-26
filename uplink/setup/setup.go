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
	mon = monkit.Package()

	// Error is the class of errors returned by this package
	Error = errs.Class("uplink setup")
)

// LoadEncryptionAccess loads an EncryptionAccess from the values specified in the encryption config.
func LoadEncryptionAccess(ctx context.Context, cfg uplink.EncryptionConfig) (_ *libuplink.EncryptionAccess, err error) {
	defer mon.Task()(&ctx)(&err)

	if cfg.EncAccessFilepath != "" {
		data, err := ioutil.ReadFile(cfg.EncAccessFilepath)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		return libuplink.ParseEncryptionAccess(strings.TrimSpace(string(data)))
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
	return libuplink.NewEncryptionAccessWithDefaultKey(*key), nil
}
