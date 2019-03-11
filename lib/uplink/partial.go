// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import "context"

// NewPartialUpload starts a new partial upload and returns that partial
// upload id
func (s *Session) NewPartialUpload(ctx context.Context, bucket string) (
	uploadID string, err error) {
	panic("TODO")
}

// TODO: lists upload ids
func (s *Session) ListPartialUploads() {
	panic("TODO")
}

// TODO: adds a new segment with given RS and node selection config
func (s *Session) PutPartialUpload() {
	panic("TODO")
}

// TODO: takes a path, metadata, etc, and puts all of the segment metadata
// into place. the object doesn't show up until this method is called.
func (s *Session) FinishPartialUpload() {
	panic("TODO")
}

// AbortPartialUpload cancels an existing partial upload.
func (s *Session) AbortPartialUpload(ctx context.Context,
	bucket, uploadID string) error {
	panic("TODO")
}
