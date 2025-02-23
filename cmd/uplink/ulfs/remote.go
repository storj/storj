// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulloc"
	"storj.io/uplink"
	"storj.io/uplink/private/object"
)

// Remote implements something close to a filesystem but backed by an uplink project.
type Remote struct {
	project *uplink.Project
}

// NewRemote returns something close to a filesystem and returns objects using the project.
func NewRemote(project *uplink.Project) *Remote {
	return &Remote{
		project: project,
	}
}

// Close releases any resources that the Remote contains.
func (r *Remote) Close() error {
	return r.project.Close()
}

// Open returns a MultiReadHandle for the object identified by a given bucket and key.
func (r *Remote) Open(ctx context.Context, bucket, key string) (MultiReadHandle, error) {
	return newUplinkMultiReadHandle(r.project, bucket, key), nil
}

// Stat returns information about an object at the specified key.
func (r *Remote) Stat(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	fstat, err := r.project.StatObject(ctx, bucket, key)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	stat := uplinkObjectToObjectInfo(bucket, fstat)
	return &stat, nil
}

// Create returns a MultiWriteHandle for the object identified by a given bucket and key.
func (r *Remote) Create(ctx context.Context, bucket, key string, opts *CreateOptions) (MultiWriteHandle, error) {
	var customMetadata uplink.CustomMetadata
	if opts.Metadata != nil {
		customMetadata = uplink.CustomMetadata(opts.Metadata)

		if err := customMetadata.Verify(); err != nil {
			return nil, err
		}
	}

	if opts.SinglePart {
		upload, err := r.project.UploadObject(ctx, bucket, key, &uplink.UploadOptions{
			Expires: opts.Expires,
		})
		if err != nil {
			return nil, err
		}
		return newUplinkSingleWriteHandle(r.project, bucket, upload, customMetadata), nil
	}

	info, err := r.project.BeginUpload(ctx, bucket, key, &uplink.UploadOptions{
		Expires: opts.Expires,
	})
	if err != nil {
		return nil, err
	}
	return newUplinkMultiWriteHandle(r.project, bucket, info, customMetadata), nil
}

// Move moves object to provided key and bucket.
func (r *Remote) Move(ctx context.Context, oldbucket, oldkey, newbucket, newkey string) error {
	return errs.Wrap(r.project.MoveObject(ctx, oldbucket, oldkey, newbucket, newkey, nil))
}

// Copy copies object to provided key and bucket.
func (r *Remote) Copy(ctx context.Context, oldbucket, oldkey, newbucket, newkey string) error {
	_, err := r.project.CopyObject(ctx, oldbucket, oldkey, newbucket, newkey, nil)
	return errs.Wrap(err)
}

// Remove deletes the object at the provided key and bucket.
func (r *Remote) Remove(ctx context.Context, bucket, key string, opts *RemoveOptions) (err error) {
	if !opts.isPending() {
		if opts.Version != nil || opts.BypassGovernanceRetention {
			_, err = object.DeleteObject(ctx, r.project, bucket, key, opts.Version, &object.DeleteObjectOptions{
				BypassGovernanceRetention: opts.BypassGovernanceRetention,
			})
		} else {
			_, err = r.project.DeleteObject(ctx, bucket, key)
		}
		if err != nil {
			return errs.Wrap(err)
		}
		return nil
	}

	// TODO: we may need a dedicated endpoint for deleting pending object streams
	list := r.project.ListUploads(ctx, bucket, &uplink.ListUploadsOptions{Prefix: key})

	// TODO: modify when we can have several pending objects for the same object key
	if list.Next() {
		err := r.project.AbortUpload(ctx, bucket, key, list.Item().UploadID)
		if err != nil {
			return errs.Wrap(err)
		}
	}
	if err := list.Err(); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// List lists all of the objects in some bucket that begin with the given prefix.
func (r *Remote) List(ctx context.Context, bucket, prefix string, opts *ListOptions) ObjectIterator {
	parentPrefix := ""
	if idx := strings.LastIndexByte(prefix, '/'); idx >= 0 {
		parentPrefix = prefix[:idx+1]
	}

	trim := ulloc.NewRemote(bucket, "")
	if !opts.isRecursive() {
		trim = ulloc.NewRemote(bucket, parentPrefix)
	}

	var iter ObjectIterator
	switch {
	case opts.isPending():
		iter = newUplinkUploadIterator(
			bucket,
			r.project.ListUploads(ctx, bucket, &uplink.ListUploadsOptions{
				Prefix:    parentPrefix,
				Recursive: opts.isRecursive(),
				System:    true,
				Custom:    opts.isExpanded(),
			}),
		)
	case opts.allVersions():
		iter = newVersionedUplinkObjectIterator(ctx, r.project, versionedUplinkObjectIteratorConfig{
			Bucket:    bucket,
			Prefix:    parentPrefix,
			Recursive: opts.isRecursive(),
			Custom:    opts.isExpanded(),
		})
	default:
		iter = newUplinkObjectIterator(
			bucket,
			r.project.ListObjects(ctx, bucket, &uplink.ListObjectsOptions{
				Prefix:    parentPrefix,
				Recursive: opts.isRecursive(),
				System:    true,
				Custom:    opts.isExpanded(),
			}),
		)
	}

	return &filteredObjectIterator{
		trim:   trim,
		filter: ulloc.NewRemote(bucket, prefix),
		iter:   iter,
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
func (u *uplinkObjectIterator) Item() ObjectInfo {
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
func (u *uplinkUploadIterator) Item() ObjectInfo {
	return uplinkUploadInfoToObjectInfo(u.bucket, u.iter.Item())
}

// Ensure that versionedUplinkObjectIterator implements ObjectIterator.
var _ ObjectIterator = (*versionedUplinkObjectIterator)(nil)

// versionedUplinkObjectIterator is an ObjectIterator that iterates over object versions.
type versionedUplinkObjectIterator struct {
	ctx     context.Context
	project *uplink.Project
	config  versionedUplinkObjectIteratorConfig

	items   []*object.VersionedObject
	itemIdx int
	cursor  struct {
		key     string
		version []byte
	}
	listingDone bool
	done        bool
	err         error
}

// versionedUplinkObjectIteratorConfig contains configuration options for a versionedUplinkObjectIterator.
type versionedUplinkObjectIteratorConfig struct {
	Bucket    string
	Prefix    string
	Recursive bool
	Custom    bool
}

// newVersionedUplinkObjectIterator returns a new versionedUplinkObjectIterator.
func newVersionedUplinkObjectIterator(ctx context.Context, project *uplink.Project, config versionedUplinkObjectIteratorConfig) *versionedUplinkObjectIterator {
	return &versionedUplinkObjectIterator{
		ctx:     ctx,
		project: project,
		config:  config,
	}
}

// Next tries to advance the iterator to the next item and returns true if it was successful.
func (iter *versionedUplinkObjectIterator) Next() bool {
	if iter.done {
		return false
	}

	if iter.itemIdx >= len(iter.items)-1 {
		if iter.listingDone {
			iter.done = true
			return false
		}

		var more bool
		iter.items, more, iter.err = object.ListObjectVersions(iter.ctx, iter.project, iter.config.Bucket, &object.ListObjectVersionsOptions{
			Prefix:        iter.config.Prefix,
			Recursive:     iter.config.Recursive,
			System:        true,
			Custom:        iter.config.Custom,
			Cursor:        iter.cursor.key,
			VersionCursor: iter.cursor.version,
		})
		iter.listingDone = !more

		if iter.err != nil || len(iter.items) == 0 {
			iter.done = true
			return false
		}

		lastItem := iter.items[len(iter.items)-1]
		iter.cursor.key = lastItem.Key
		iter.cursor.version = lastItem.Version

		iter.itemIdx = 0
		return true
	}

	iter.itemIdx++
	return true
}

// Item returns the current item in the iteration.
// It panics if Next() was not called or if its return value was ignored.
func (iter *versionedUplinkObjectIterator) Item() ObjectInfo {
	if iter.done {
		panic("iteration is done")
	}
	if iter.itemIdx >= len(iter.items) {
		panic("iteration is out of bounds")
	}

	item := iter.items[iter.itemIdx]
	info := uplinkObjectToObjectInfo(iter.config.Bucket, &item.Object)
	info.Loc = ulloc.NewRemote(iter.config.Bucket, iter.config.Prefix+item.Key)
	info.Version = item.Version
	info.IsDeleteMarker = item.IsDeleteMarker

	return info
}

// Err returns the error encountered during iteration.
func (iter *versionedUplinkObjectIterator) Err() error {
	return iter.err
}
