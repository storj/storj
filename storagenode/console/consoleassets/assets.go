// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleassets

import (
	"net/http"
)

// FileSystem is nil by default, but when our Makefile generates
// embedded resources via go-bindata, it will also drop an init method
// that sets this to not nil.
var FileSystem http.FileSystem
