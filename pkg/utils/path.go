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
	Local  bool
	Schema string
	Path   string
}

// Creates new Struct from handed over URL
func (p *FPath) New(url string) {

	// Check for Schema
	sepstring := strings.Split(url, "://")

	switch len(sepstring) {
	case 2: // Has Schema
		p.Schema = sepstring[0]
		p.Path = sepstring[1]
	case 1: // No Schema
		p.Local = true
		p.Path = sepstring[0]
	default: // Everything else is misformatted
		log.Fatalf("misformatted URL: %v", url)
	}

	// Check for Windows Special Handling Prefix
	cprefix, _ := regexp.Compile(`^\\\\\?\\(UNC\\)?`)

	// when Prefix present, omit further changes to the path
	if prefix := cprefix.FindString(p.Path); prefix != "" {
		p.Schema = prefix
		p.Path = strings.Replace(p.Path, prefix, "", -1) //Strip Prefix
	} else {
		// when file is local, ensure path absolute
		if p.IsLocal() && !filepath.IsAbs(p.Path) {
			p.Path, _ = filepath.Abs(p.Path)
		}
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
	fileInfo, err := os.Stat(p.Path)
	if err != nil {
		//fmt.Println(err)
		return false
	}
	return fileInfo.IsDir()
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
		if p.Schema != "" {
			cpl = p.Schema + "://" + p.Path
		} else {
			cpl = p.Path
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
