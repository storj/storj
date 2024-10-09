// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package dbx

import (
	// make sure we load our cockroach driver so dbx.Open can find it.
	_ "storj.io/storj/shared/dbutil/cockroachutil"
)
