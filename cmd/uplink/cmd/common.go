// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"storj.io/common/fpath"
	"storj.io/storj/pkg/cfgstruct"
)

func getConfDir() string {
	if param := cfgstruct.FindConfigDirParam(); param != "" {
		return param
	}
	return fpath.ApplicationDir("storj", "uplink")
}
