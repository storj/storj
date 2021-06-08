// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/uplink"
)

// filesystemRemote implements something close to a filesystem but backed by an uplink project.
type filesystemRemote struct {
	project *uplink.Project
}

func (r *filesystemRemote) Close() error {
	return r.project.Close()
}

func (r *filesystemRemote) Open(ctx context.Context, bucket, key string) (readHandle, error) {
	fh, err := r.project.DownloadObject(ctx, bucket, key, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return newUplinkReadHandle(bucket, fh), nil
}

func (r *filesystemRemote) Create(ctx context.Context, bucket, key string) (writeHandle, error) {
	fh, err := r.project.UploadObject(ctx, bucket, key, nil)
	if err != nil {
		return nil, err
	}
	return newUplinkWriteHandle(fh), nil
}

func (r *filesystemRemote) ListObjects(ctx context.Context, bucket, prefix string, recursive bool) objectIterator {
	parentPrefix := ""
	if idx := strings.LastIndexByte(prefix, '/'); idx >= 0 {
		parentPrefix = prefix[:idx+1]
	}

	trim := parentPrefix
	if recursive {
		trim = ""
	}

	return &filteredObjectIterator{
		trim:   trim,
		filter: prefix,
		iter: newUplinkObjectIterator(bucket, r.project.ListObjects(ctx, bucket,
			&uplink.ListObjectsOptions{
				Prefix:    parentPrefix,
				Recursive: recursive,
				System:    true,
			})),
	}
}

func (r *filesystemRemote) ListPendingMultiparts(ctx context.Context, bucket, prefix string, recursive bool) objectIterator {
	parentPrefix := ""
	if idx := strings.LastIndexByte(prefix, '/'); idx >= 0 {
		parentPrefix = prefix[:idx+1]
	}

	trim := parentPrefix
	if recursive {
		trim = ""
	}

	return &filteredObjectIterator{
		trim:   trim,
		filter: prefix,
		iter: newUplinkUploadIterator(bucket, r.project.ListUploads(ctx, bucket,
			&uplink.ListUploadsOptions{
				Prefix:    parentPrefix,
				Recursive: recursive,
				System:    true,
			})),
	}
}

// uplinkObjectIterator implements objectIterator for *uplink.ObjectIterator.
type uplinkObjectIterator struct {
	bucket string
	iter   *uplink.ObjectIterator
}

// newUplinkObjectIterator constructs an *uplinkObjectIterator from an *uplink.ObjectIterator.
func newUplinkObjectIterator(bucket string, iter *uplink.ObjectIterator) *uplinkObjectIterator {
	return &uplinkObjectIterator{
		bucket: bucket,
		iter:   iter,
	}
}

func (u *uplinkObjectIterator) Next() bool { return u.iter.Next() }
func (u *uplinkObjectIterator) Err() error { return u.iter.Err() }
func (u *uplinkObjectIterator) Item() objectInfo {
	return uplinkObjectToObjectInfo(u.bucket, u.iter.Item())
}

// uplinkUploadIterator implements objectIterator for *multipart.UploadIterators.
type uplinkUploadIterator struct {
	bucket string
	iter   *uplink.UploadIterator
}

// newUplinkUploadIterator constructs a *uplinkUploadIterator from a *uplink.UploadIterator.
func newUplinkUploadIterator(bucket string, iter *uplink.UploadIterator) *uplinkUploadIterator {
	return &uplinkUploadIterator{
		bucket: bucket,
		iter:   iter,
	}
}

func (u *uplinkUploadIterator) Next() bool { return u.iter.Next() }
func (u *uplinkUploadIterator) Err() error { return u.iter.Err() }
func (u *uplinkUploadIterator) Item() objectInfo {
	return uplinkUploadInfoToObjectInfo(u.bucket, u.iter.Item())
}
