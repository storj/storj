// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset

import (
	"net/http"
)

// FileSystem creates an http.FileSystem out of dir.
func FileSystem(dir string) http.FileSystem {
	return http.Dir(dir)
}
