// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// Package ulext provides an interface for the CLI to interface with the external world.
package ulext

import (
	"context"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/uplink"
)

// External is the interface for all of the ways that the uplink command may interact with
// any external state.
type External interface {
	OpenFilesystem(ctx context.Context, accessName string, options ...Option) (ulfs.Filesystem, error)
	OpenProject(ctx context.Context, accessName string, options ...Option) (*uplink.Project, error)

	AccessInfoFile() string
	OpenAccess(accessName string) (access *uplink.Access, err error)
	GetAccessInfo(required bool) (string, map[string]string, error)
	SaveAccessInfo(defaultName string, accesses map[string]string) error
	RequestAccess(ctx context.Context, satelliteAddress, apiKey, passphrase string) (*uplink.Access, error)
	ExportAccess(ctx clingy.Context, access *uplink.Access, filename string) error

	ConfigFile() string
	SaveConfig(values map[string]string) error

	PromptInput(ctx clingy.Context, prompt string) (input string, err error)
	PromptSecret(ctx clingy.Context, prompt string) (secret string, err error)
}

// Options contains all of the possible options for opening a filesystem or project.
type Options struct {
	EncryptionBypass bool
}

// LoadOptions takes a slice of Option values and returns a filled out Options struct.
func LoadOptions(options ...Option) (opts Options) {
	for _, opt := range options {
		opt.apply(&opts)
	}
	return opts
}

// Option is a single option that controls the Options struct.
type Option struct {
	apply func(*Options)
}

// BypassEncryption will disable decrypting of path names if bypass is true.
func BypassEncryption(bypass bool) Option {
	return Option{apply: func(opt *Options) { opt.EncryptionBypass = bypass }}
}
