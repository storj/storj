// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"github.com/zeebo/errs"
)

// ErrNoPointers is the pdbclient error class
var ErrNoPointers = errs.New("pointer error: no pointers exist")
