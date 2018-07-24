// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storage

import (
	"storj.io/storj/pkg/paths"
)

// ListItem is a single item in a listing
type ListItem struct {
	Path paths.Path
	Meta Meta
}
