// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair
import (
	// "storj.io/storj/pkg/pb"
)

//Queue ...
type Queue interface {
	Add()
	Remove()
	GetOffline()
}