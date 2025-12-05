// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import "os"

// FileExistsConfig contains the configuration for FileExists.
type FileExistsConfig struct {
	Path string `help:"path to the file. Cleanup will be stopped if file exists" default:"/tmp/storj.chore.disable"`
}

// FileExists is an availability check which is false if file exists.
type FileExists struct {
	Path string
}

var _ Enablement = (*FileExists)(nil)

// NewFileExists creates a new FileExists.
func NewFileExists(config FileExistsConfig) *FileExists {
	return &FileExists{
		Path: config.Path,
	}
}

// Enabled implements Enablement.
func (f *FileExists) Enabled() (bool, error) {
	_, err := os.Stat(f.Path)
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
}
