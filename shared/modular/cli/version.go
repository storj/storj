// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"fmt"

	"storj.io/common/version"
)

// Version is a command that outputs the build version of the application.
type Version struct {
}

// NewVersion creates a new Version command.
func NewVersion() *Version {
	return &Version{}
}

// Run outputs the build version of the application.
func (v Version) Run() error {
	fmt.Println(version.Build)
	return nil
}
