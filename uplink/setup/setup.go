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
func LoadEncryptionAccess(ctx context.Context, cfg uplink.Legacy) (_ *libuplink.EncryptionAccess, err error) {
	defer mon.Task()(&ctx)(&err)

	if cfg.Enc.EncAccessFilepath != "" {
		data, err := ioutil.ReadFile(cfg.Enc.EncAccessFilepath)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		return libuplink.ParseEncryptionAccess(strings.TrimSpace(string(data)))
	}

	data := []byte(cfg.Enc.EncryptionKey)
	if cfg.Enc.KeyFilepath != "" {
		data, err = ioutil.ReadFile(cfg.Enc.KeyFilepath)
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
