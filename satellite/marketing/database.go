// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package marketing

// DB contains access to all marketing related databases
type DB interface {
	Offers() Offers
}
