// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"strings"
)

// Path is the unique identifer of file paths
type Path []string

// This function transforms the string path of type []string to a []byte type
// given []string{"path/one/is/here"}
func (path *Path) Bytes() []byte {
	stringPath := strings.Join(*path, " ")
	bytesPath := []byte(stringPath)
	return bytesPath
}


