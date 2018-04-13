// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"github.com/zeebo/errs"
)

// UnexpectedStatusCode is an error class for unexpected HTTP response
var UnexpectedStatusCode = errs.Class("unexpected status code")

// CryptoError is an error class for encryption errors
var CryptoError = errs.Class("encryption error")
