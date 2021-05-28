// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// appDir returns best base directory for the currently running operating system. It
// has a legacy bool to have it return the same values that storj.io/common/fpath.ApplicationDir
// would have returned.
func appDir(legacy bool, subdir ...string) string {
	for i := range subdir {
		if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
			subdir[i] = strings.Title(subdir[i])
		} else {
			subdir[i] = strings.ToLower(subdir[i])
		}
	}
	var appdir string
	home := os.Getenv("HOME")

	switch runtime.GOOS {
	case "windows":
		// Windows standards: https://msdn.microsoft.com/en-us/library/windows/apps/hh465094.aspx?f=255&MSPPError=-2147217396
		for _, env := range []string{"AppData", "AppDataLocal", "UserProfile", "Home"} {
			val := os.Getenv(env)
			if val != "" {
				appdir = val
				break
			}
		}
	case "darwin":
		// Mac standards: https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/FileSystemProgrammingGuide/MacOSXDirectories/MacOSXDirectories.html
		appdir = filepath.Join(home, "Library", "Application Support")
	case "linux":
		fallthrough
	default:
		// Linux standards: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
		if legacy {
			appdir = os.Getenv("XDG_DATA_HOME")
			if appdir == "" && home != "" {
				appdir = filepath.Join(home, ".local", "share")
			}
		} else {
			appdir = os.Getenv("XDG_CONFIG_HOME")
			if appdir == "" && home != "" {
				appdir = filepath.Join(home, ".config")
			}
		}
	}
	return filepath.Join(append([]string{appdir}, subdir...)...)
}
