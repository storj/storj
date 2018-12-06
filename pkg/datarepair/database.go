// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package datarepair

// DB contains access to different datarepair databases
type DB interface {
	// Users is a getter for Users repository
	IrreparableDB() IrreparableDB
}
