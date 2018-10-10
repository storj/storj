// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"sync"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

// Auditor is an auditor
type Auditor struct {
	pointers   pdbclient.Client
	lastPath   *paths.Path
	mutex      sync.Mutex
	downloader downloader
}

// NewAuditor creates a new instance of audit
func NewAuditor(pointers pdbclient.Client, downloader downloader) *Auditor {
	return &Auditor{pointers: pointers, downloader: downloader}
}

// defaultDownloader implements the downloader interface
//nolint - defaultDownloader isn't called in tests
type defaultDownloader struct {
	transport transport.Client
	overlay   overlay.Client
	identity  provider.FullIdentity
}

// newDefaultDownloader creates a new instance of a defaultDownloader struct
//nolint - newDefaultDownloader isn't called in tests
func newDefaultDownloader(t transport.Client, o overlay.Client, id provider.FullIdentity) *defaultDownloader {
	return &defaultDownloader{transport: t, overlay: o, identity: id}
}

// TODO: give more descriptive name
func (a *Auditor) auditCall(ctx context.Context) (err error) {
	newStripe, pointer, _, err := a.NextStripe(ctx)
	if err != nil {
		return err
	}
	err = a.auditStripe(ctx, pointer, newStripe.Index)
	if err != nil {
		return err
	}
	return nil
}
