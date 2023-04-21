// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"io"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/uplink"
)

//
// read handles
//

type uplinkMultiReadHandle struct {
	project *uplink.Project
	bucket  string
	key     string

	mu   sync.Mutex
	done bool
	eof  bool
	off  int64
	info *ObjectInfo
}

func newUplinkMultiReadHandle(project *uplink.Project, bucket, key string) *uplinkMultiReadHandle {
	return &uplinkMultiReadHandle{
		project: project,
		bucket:  bucket,
		key:     key,
	}
}

func (u *uplinkMultiReadHandle) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.done = true

	return nil
}

func (u *uplinkMultiReadHandle) SetOffset(offset int64) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.done {
		return errs.New("already closed")
	}

	u.off = offset
	u.eof = false

	return nil
}

func (u *uplinkMultiReadHandle) NextPart(ctx context.Context, length int64) (ReadHandle, error) {
	opts, err := func() (opts *uplink.DownloadOptions, err error) {
		u.mu.Lock()
		defer u.mu.Unlock()

		if u.done {
			return nil, errs.New("already closed")
		} else if u.eof {
			return nil, io.EOF
		} else if u.info != nil && u.off >= u.info.ContentLength {
			return nil, io.EOF
		}

		opts = &uplink.DownloadOptions{Offset: u.off, Length: length}
		if u.off < 0 {
			opts.Length = -1
			u.eof = u.off+length > 0
		}
		u.off += length

		return opts, nil
	}()
	if err != nil {
		return nil, err
	}

	// TODO: this can cause tearing if the object is modified during
	// the download. this should be fixed when we extend the api to
	// allow requesting a specific version of the object.
	dl, err := u.project.DownloadObject(ctx, u.bucket, u.key, opts)
	if err != nil {
		return nil, err
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.info == nil {
		info := uplinkObjectToObjectInfo(u.bucket, dl.Info())
		u.info = &info
	}

	if u.off < 0 {
		if norm := u.off + u.info.ContentLength; norm > 0 {
			u.off = norm
		}
	}

	return &uplinkReadHandle{
		info: u.info,
		dl:   dl,
	}, nil
}

func (u *uplinkMultiReadHandle) Info(ctx context.Context) (*ObjectInfo, error) {
	u.mu.Lock()
	if u.info != nil {
		u.mu.Unlock()
		return u.info, nil
	}
	u.mu.Unlock()

	// TODO(jeff): maybe we want to dedupe concurrent requests?

	obj, err := u.project.StatObject(ctx, u.bucket, u.key)
	if err != nil {
		return nil, err
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if u.info == nil {
		info := uplinkObjectToObjectInfo(u.bucket, obj)
		u.info = &info
	}

	info := *u.info
	return &info, nil
}

// Length returns the size of the object.
func (u *uplinkMultiReadHandle) Length() int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	// if we have not fetched the info yet, return unknown length
	if u.info == nil {
		return -1
	}
	return u.info.ContentLength
}

// uplinkReadHandle implements readHandle for *uplink.Downloads.
type uplinkReadHandle struct {
	info *ObjectInfo
	dl   *uplink.Download
}

func (u *uplinkReadHandle) Read(p []byte) (int, error) { return u.dl.Read(p) }
func (u *uplinkReadHandle) Close() error               { return u.dl.Close() }
func (u *uplinkReadHandle) Info() ObjectInfo           { return *u.info }

//
// write handles
//

type uplinkWriteHandle struct {
	upload *uplink.Upload
}

func newUplinkWriteHandle(upload *uplink.Upload) *uplinkWriteHandle {
	return &uplinkWriteHandle{
		upload: upload,
	}
}

func (u *uplinkWriteHandle) Write(b []byte) (int, error) { return u.upload.Write(b) }
func (u *uplinkWriteHandle) Commit() error               { return u.upload.Commit() }
func (u *uplinkWriteHandle) Abort() error                { return u.upload.Abort() }
