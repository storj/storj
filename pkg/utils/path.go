// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"log"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

type FPath struct {
	Local      bool
	Delimiter  byte
	Schema     string
	Path       string
	VolumeName string
}

// Creates new Struct from handed over URL
func (p FPath) New(url url.URL) {

	// Check for Schema
	sepstring := strings.Split(url.String(), "://")

	switch len(sepstring) {
	case 2: // Has Schema
		p.Schema = sepstring[0]
		p.Path = sepstring[1]
	case 1: // No Schema
		p.Path = sepstring[0]
	default:
		log.Fatalf("misformatted URL: %v", url.String())
	}

	// Check for Windows Volumename in p.Path
	p.VolumeName = filepath.VolumeName(p.Path)
	if p.VolumeName != "" {
		strings.Replace(p.Path, p.VolumeName, "", 1)
	}

}

// Joins/appends segment to the path
func (p FPath) Join(segment string) FPath {
	p.Path = filepath.Join(p.Path, segment)
	return p
}

// Returns the last folder of Path
func (p FPath) Folder() string {
	return filepath.Dir(p.Path)
}

// Returns if Path is a folder
func (p FPath) IsFolder() bool {
	return false
}

// Returns Base of Path
func (p FPath) Base() string {
	return filepath.Base(p.Path)
}

// Returns whether URL refers to local or remote location
func (p FPath) IsLocal() bool {
	return p.Local
}

// Returns if URL had a schema
func (p FPath) HasSchema() bool {
	return p.Schema != ""
}

// Returns entire URL
func (p FPath) String() string {
	var cpl string

	switch runtime.GOOS {
	case "windows":
		if p.Local {
			cpl = p.VolumeName + p.Path // C:/data/upload.txt
		} else if p.Schema != "" {
			cpl = p.Schema + "://" + p.Path // redis://127.0.0.1
		} else {
			cpl = "\\" + p.Path // \\fileserver\data\upload.txt
			//cpl = strings.Replace(cpl,"/","", -1)
		}

	default:
		if p.Schema != "" {
			cpl = p.Schema + "://" + p.Path
		} else {
			cpl = p.Path
		}
	}
	return cpl
}
