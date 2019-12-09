// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
)

// Error messages
const (
	internalErrDetailedMsg = "It looks like we had a problem on our end. Please try again"
	projectLimitErrMsg     = "Sorry, during the Vanguard release you have a limited number of projects"
)

// errConsoleInternalDetailed describes detailed error message for internal error
var errConsoleInternalDetailed = errs.New(internalErrDetailedMsg)

var errProjectLimit = errs.New(projectLimitErrMsg)

// HandleError returns detailed error if such error handles
func HandleError(err error) error {
	switch {
	case console.Error.Has(err):
		return errConsoleInternalDetailed
	case console.ErrProjLimit.Has(err):
		return errProjectLimit
	default:
		return err
	}
}
