// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package ulext

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/uplink"
)

// MethodUnsupported is an error class indicating that a method is unsupported.
var MethodUnsupported = errs.Class("method unsupported")

// ExternalUnsupported is a stub implementation of ulext.External.
// All method calls return an instance of the error class MethodUnsupported.
type ExternalUnsupported struct{}

// OpenFilesystem is a stub implementation of (ulext.External).OpenFilesystem.
func (ex *ExternalUnsupported) OpenFilesystem(ctx context.Context, accessName string, options ...Option) (ulfs.Filesystem, error) {
	return nil, MethodUnsupported.New("")
}

// OpenProject is a stub implementation of (ulext.External).OpenProject.
func (ex *ExternalUnsupported) OpenProject(ctx context.Context, accessName string, options ...Option) (*uplink.Project, error) {
	return nil, MethodUnsupported.New("")
}

// GetEdgeUrlOverrides is a stub implementation of (ulext.External).GetEdgeUrlOverrides.
func (ex *ExternalUnsupported) GetEdgeUrlOverrides(ctx context.Context, access *uplink.Access) (EdgeURLOverrides, error) {
	return EdgeURLOverrides{}, MethodUnsupported.New("")
}

// AccessInfoFile is a stub implementation of (ulext.External).AccessInfoFile.
func (ex *ExternalUnsupported) AccessInfoFile() (string, error) {
	return "", MethodUnsupported.New("")
}

// OpenAccess is a stub implementation of (ulext.External).OpenAccess.
func (ex *ExternalUnsupported) OpenAccess(accessName string) (*uplink.Access, error) {
	return nil, MethodUnsupported.New("")
}

// GetAccessInfo is a stub implementation of (ulext.External).GetAccessInfo.
func (ex *ExternalUnsupported) GetAccessInfo(required bool) (string, map[string]string, error) {
	return "", nil, MethodUnsupported.New("")
}

// SaveAccessInfo is a stub implementation of (ulext.External).SaveAccessInfo.
func (ex *ExternalUnsupported) SaveAccessInfo(defaultName string, accesses map[string]string) error {
	return MethodUnsupported.New("")
}

// RequestAccess is a stub implementation of (ulext.External).RequestAccess.
func (ex *ExternalUnsupported) RequestAccess(ctx context.Context, satelliteAddress, apiKey, passphrase string, unencryptedObjectKeys bool) (*uplink.Access, error) {
	return nil, MethodUnsupported.New("")
}

// ExportAccess is a stub implementation of (ulext.External).ExportAccess.
func (ex *ExternalUnsupported) ExportAccess(ctx context.Context, access *uplink.Access, filename string) error {
	return MethodUnsupported.New("")
}

// ConfigFile is a stub implementation of (ulext.External).ConfigFile.
func (ex *ExternalUnsupported) ConfigFile() (string, error) {
	return "", MethodUnsupported.New("")
}

// SaveConfig is a stub implementation of (ulext.External).SaveConfig.
func (ex *ExternalUnsupported) SaveConfig(values map[string]string) error {
	return MethodUnsupported.New("")
}

// PromptInput is a stub implementation of (ulext.External).PromptInput.
func (ex *ExternalUnsupported) PromptInput(ctx context.Context, prompt string) (string, error) {
	return "", MethodUnsupported.New("")
}

// PromptSecret is a stub implementation of (ulext.External).PromptSecret.
func (ex *ExternalUnsupported) PromptSecret(ctx context.Context, prompt string) (string, error) {
	return "", MethodUnsupported.New("")
}
