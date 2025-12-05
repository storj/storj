// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"github.com/spacemonkeygo/monkit/v3"
)

const (
	contentType = "Content-Type"

	applicationJSON = "application/json"
)

var mon = monkit.Package()
