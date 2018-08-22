// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package conf

import (
	"os"
	"path/filepath"
	"runtime"
)

func DefaultDir(subpaths ...string) string {
	var dir string
	switch runtime.GOOS {
	default:
		dir = "$HOME/.storj/capt"
	case "windows":
		dir = filepath.Join(os.Getenv("AppData"), "Storj", "capt")
	}

	return filepath.Join(append([]string{dir}, subpaths...)...)
}
