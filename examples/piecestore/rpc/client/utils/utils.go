// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// DetermineID -- Get the id for a section of data
func DetermineID(f *os.File, offset int64, length int64) (string, error) {
	h := sha256.New()

	fSection := io.NewSectionReader(f, offset, length)
	if _, err := io.Copy(h, fSection); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
