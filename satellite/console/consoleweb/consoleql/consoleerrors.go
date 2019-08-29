// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
)

const (
	internalErrDetailedMsg = "It looks like we had a problem on our end. Please try again"
)

var ErrConsoleInternalDetailed = errs.New(internalErrDetailedMsg)

func HandleError(err error) error {
	if console.ErrConsoleInternal.Has(err) {
		return ErrConsoleInternalDetailed
	}

	return err
}
