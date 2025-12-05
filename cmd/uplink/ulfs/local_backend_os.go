// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import "os"

// LocalBackendOS implements LocalBackend by using the os package.
type LocalBackendOS struct{}

// NewLocalBackendOS constructs a new LocalBackendOS.
func NewLocalBackendOS() *LocalBackendOS {
	return new(LocalBackendOS)
}

// Create calls os.Create.
func (l *LocalBackendOS) Create(name string) (LocalBackendFile, error) {
	return os.Create(name)
}

// MkdirAll calls os.MkdirAll.
func (l *LocalBackendOS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Open calls os.Open.
func (l *LocalBackendOS) Open(name string) (LocalBackendFile, error) {
	return os.Open(name)
}

// Remove calls os.Remove.
func (l *LocalBackendOS) Remove(name string) error {
	return os.Remove(name)
}

// Rename calls os.Rename.
func (l *LocalBackendOS) Rename(oldname, newname string) error {
	return os.Rename(oldname, newname)
}

// Stat calls os.Stat.
func (l *LocalBackendOS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
