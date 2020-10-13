// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package dbx

import (
	"github.com/spacemonkeygo/monkit/v3"
)

//go:generate sh gen.sh

var mon = monkit.Package()
