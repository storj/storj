// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type FPath struct {
	local  bool
	schema string
	path   string
}

// New creates Struct from handed over URL
func (p *FPath) New(url string) {

	// Check for Schema
	sepstring := strings.Split(url, "://")

	switch len(sepstring) {
	case 2: // Has Schema
		p.schema = sepstring[0]
		p.path = sepstring[1]
	case 1: // No Schema
		p.local = true
		p.path = sepstring[0]
	default: // Everything else is misformatted
		log.Fatalf("misformatted URL: %v", url)
	}

	// Check for Windows Special Handling Prefix
	cprefix, _ := regexp.Compile(`^\\\\\?\\(UNC\\)?`)

	// when Prefix present, omit further changes to the path
	if prefix := cprefix.FindString(p.path); prefix != "" {
		p.schema = prefix
		p.path = strings.Replace(p.path, prefix, "", -1) //Strip Prefix
	} else {
		// when file is local, ensure path absolute
		if p.IsLocal() && !filepath.IsAbs(p.path) {
			p.path, _ = filepath.Abs(p.path)
		}
	}
}

// Join is merging segment to the path
func (p FPath) Join(segment string) FPath {
	p.path = filepath.Join(p.path, segment)
	return p
}

// Folder returns the last folder of path
func (p FPath) Folder() string {
	return filepath.Dir(p.path)
}

// IsFolder returns if path is a folder
func (p FPath) IsFolder() bool {
	fileInfo, err := os.Stat(p.path)
	if err != nil {
		//fmt.Println(err)
		return false
	}
	return fileInfo.IsDir()
}

// Base returns Base Segment of the path
func (p FPath) Base() string {
	return filepath.Base(p.path)
}

// IsLocal returns whether URL refers to local or remote location
func (p FPath) IsLocal() bool {
	return p.local
}

// HasSchema returns if URL had a schema
func (p FPath) HasSchema() bool {
	return p.schema != ""
}

// Schema returns the schema if existing
func (p FPath) Schema() string {
	return p.schema
}

// Path returns the URL path without schema
func (p FPath) Path() string {
	return p.path
}

// String returns entire URL
func (p FPath) String() string {
	var cpl string

	switch runtime.GOOS {
	case "windows":
		if p.schema != "" {
			cpl = p.schema + "://" + p.path
		} else {
			cpl = p.path
		}
	default:
		if p.schema != "" {
			cpl = p.schema + "://" + p.path
		} else {
			cpl = p.path
		}
	}
	return cpl
}
