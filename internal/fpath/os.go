// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"storj.io/storj/pkg/utils"
)

// IsRoot returns whether path is the root directory
func IsRoot(path string) bool {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}

	return filepath.Dir(path) == path
}

// ApplicationDir returns best base directory for specific OS
func ApplicationDir(subdir ...string) string {
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
		appdir = os.Getenv("XDG_DATA_HOME")
		if appdir == "" && home != "" {
			appdir = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(append([]string{appdir}, subdir...)...)
}

// IsValidSetupDir checks if directory is valid for setup configuration
func IsValidSetupDir(name string) (ok bool, err error) {
	_, err = os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return true, err
		}
		return false, err
	}

	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer func() {
		err = utils.CombineErrors(err, f.Close())
	}()

	for {
		var filenames []string
		filenames, err = f.Readdirnames(100)
		if err == io.EOF {
			// nothing more
			return true, nil
		} else if err != nil {
			// something went wrong
			return false, err
		}

		for _, filename := range filenames {
			if filename == "config.yaml" {
				return false, nil
			}
		}
	}
}
