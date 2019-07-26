// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

// NoRowsError is a representation of specific error from database layer
type NoRowsError struct {
	message string
}

func (n NoRowsError) Error() string {
	return n.message
}
