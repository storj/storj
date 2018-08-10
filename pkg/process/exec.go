// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"log"
	"os"
)

// check if file exists, handle error correctly if it doesn't
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatalf("failed to check for file existence: %v", err)
	}
	return true
}
