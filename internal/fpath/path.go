// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Create a set
var storjScheme = map[string]struct{}{
	"sj": {},
	"s3": {},
}

// FPath is an OS independently path handling structure
type FPath struct {
	original string // the original URL or local path
	local    bool   // if local path
	bucket   string // only for Storj URL
	path     string // only for Storj URL - the path within the bucket, cleaned from duplicated slashes
}

// New creates new FPath from the given URL
func New(p string) (FPath, error) {
	fp := FPath{original: p}

	if filepath.IsAbs(p) {
		fp.local = true
		return fp, nil
	}

	var u *url.URL
	var err error
	for {
		u, err = url.Parse(p)
		if err != nil {
			return fp, fmt.Errorf("malformed URL: %v, use format sj://bucket/", err)
		}

		if u.Scheme == "" {
			fp.local = true
			return fp, nil
		}

		if _, validScheme := storjScheme[u.Scheme]; !validScheme {
			return fp, fmt.Errorf("unsupported URL scheme: %s, use format sj://bucket/", u.Scheme)
		}

		if u.Host == "" && u.Path == "" {
			return fp, errors.New("no bucket specified, use format sj://bucket/")
		}

		if u.Host != "" {
			break
		}

		p = strings.Replace(p, ":///", "://", 1)
	}

	if u.Port() != "" {
		return fp, errors.New("port in Storj URL is not supported, use format sj://bucket/")
	}

	fp.bucket = u.Host
	if u.Path != "" {
		fp.path = strings.TrimLeft(path.Clean(u.Path), "/")
	}

	return fp, nil
}

// Join is appends the given segment to the path
func (p FPath) Join(segment string) FPath {
	if p.local {
		p.original = filepath.Join(p.original, segment)
		return p
	}

	p.original += "/" + segment
	p.path = path.Join(p.path, segment)
	return p
}

// Base returns the last segment of the path
func (p FPath) Base() string {
	if p.local {
		return filepath.Base(p.original)
	}
	if p.path == "" {
		return ""
	}
	return path.Base(p.path)
}

// Bucket returns the first segment of path
func (p FPath) Bucket() string {
	return p.bucket
}

// Path returns the URL path without the scheme
func (p FPath) Path() string {
	if p.local {
		return p.original
	}
	return p.path
}

// IsLocal returns whether the path refers to local or remote location
func (p FPath) IsLocal() bool {
	return p.local
}

// String returns the entire URL (untouched)
func (p FPath) String() string {
	return p.original
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
func IsValidSetupDir(name string) (bool, error) {
	_, err := os.Stat(name)
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
	defer func() { _ = f.Close() }()

	_, err = f.Readdir(1)
	if err == io.EOF {
		// is empty
		return true, nil
	}
	return false, err
}
