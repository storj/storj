// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package strictcsv

import (
	"strings"

	"github.com/zeebo/errs"
)

var (
	// Error is an error class for the package.
	Error = errs.Class("strictcsv")
)

// parseCSVTag splits a `csv:"header[,opt...]"` tag into its header and
// recognized options. The only currently recognized option is `optional`,
// which allows the header to be absent from the CSV.
func parseCSVTag(tag, fieldName string) (header string, optional bool, err error) {
	parts := strings.Split(tag, ",")
	header = parts[0]
	for _, opt := range parts[1:] {
		switch opt {
		case "optional":
			optional = true
		default:
			return "", false, Error.New("field %q has unknown csv tag option %q", fieldName, opt)
		}
	}
	return header, optional, nil
}
