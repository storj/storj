// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulloc"
	"storj.io/uplink"
)

// Filesystem represents either the local Filesystem or the data backed by a project.
type Filesystem interface {
	Close() error
	Open(ctx clingy.Context, loc ulloc.Location) (ReadHandle, error)
	Create(ctx clingy.Context, loc ulloc.Location) (WriteHandle, error)
	Remove(ctx context.Context, loc ulloc.Location) error
	ListObjects(ctx context.Context, prefix ulloc.Location, recursive bool) (ObjectIterator, error)
	ListUploads(ctx context.Context, prefix ulloc.Location, recursive bool) (ObjectIterator, error)
	IsLocalDir(ctx context.Context, loc ulloc.Location) bool
}

//
// object info
//

// ObjectInfo is a simpler *uplink.Object that contains the minimal information the
// uplink command needs that multiple types can be converted to.
type ObjectInfo struct {
	Loc           ulloc.Location
	IsPrefix      bool
	Created       time.Time
	ContentLength int64
}

// uplinkObjectToObjectInfo returns an objectInfo converted from an *uplink.Object.
func uplinkObjectToObjectInfo(bucket string, obj *uplink.Object) ObjectInfo {
	return ObjectInfo{
		Loc:           ulloc.NewRemote(bucket, obj.Key),
		IsPrefix:      obj.IsPrefix,
		Created:       obj.System.Created,
		ContentLength: obj.System.ContentLength,
	}
}

// uplinkUploadInfoToObjectInfo returns an objectInfo converted from an *uplink.Object.
func uplinkUploadInfoToObjectInfo(bucket string, upl *uplink.UploadInfo) ObjectInfo {
	return ObjectInfo{
		Loc:           ulloc.NewRemote(bucket, upl.Key),
		IsPrefix:      upl.IsPrefix,
		Created:       upl.System.Created,
		ContentLength: upl.System.ContentLength,
	}
}

//
// read handles
//

// ReadHandle is something that can be read from.
type ReadHandle interface {
	io.Reader
	io.Closer
	Info() ObjectInfo
}

// uplinkReadHandle implements readHandle for *uplink.Downloads.
type uplinkReadHandle struct {
	bucket string
	dl     *uplink.Download
}

// newUplinkReadHandle constructs an *uplinkReadHandle from an *uplink.Download.
func newUplinkReadHandle(bucket string, dl *uplink.Download) *uplinkReadHandle {
	return &uplinkReadHandle{
		bucket: bucket,
		dl:     dl,
	}
}

func (u *uplinkReadHandle) Read(p []byte) (int, error) { return u.dl.Read(p) }
func (u *uplinkReadHandle) Close() error               { return u.dl.Close() }
func (u *uplinkReadHandle) Info() ObjectInfo           { return uplinkObjectToObjectInfo(u.bucket, u.dl.Info()) }

// osReadHandle implements readHandle for *os.Files.
type osReadHandle struct {
	raw  *os.File
	info ObjectInfo
}

// newOsReadHandle constructs an *osReadHandle from an *os.File.
func newOSReadHandle(fh *os.File) (*osReadHandle, error) {
	fi, err := fh.Stat()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &osReadHandle{
		raw: fh,
		info: ObjectInfo{
			Loc:           ulloc.NewLocal(fh.Name()),
			IsPrefix:      false,
			Created:       fi.ModTime(), // TODO: os specific crtime
			ContentLength: fi.Size(),
		},
	}, nil
}

func (o *osReadHandle) Read(p []byte) (int, error) { return o.raw.Read(p) }
func (o *osReadHandle) Close() error               { return o.raw.Close() }
func (o *osReadHandle) Info() ObjectInfo           { return o.info }

// genericReadHandle implements readHandle for an io.Reader.
type genericReadHandle struct{ r io.Reader }

// newGenericReadHandle constructs a *genericReadHandle from any io.Reader.
func newGenericReadHandle(r io.Reader) *genericReadHandle {
	return &genericReadHandle{r: r}
}

func (g *genericReadHandle) Read(p []byte) (int, error) { return g.r.Read(p) }
func (g *genericReadHandle) Close() error               { return nil }
func (g *genericReadHandle) Info() ObjectInfo           { return ObjectInfo{ContentLength: -1} }

//
// write handles
//

// WriteHandle is anything that can be written to with commit/abort semantics.
type WriteHandle interface {
	io.Writer
	Commit() error
	Abort() error
}

// uplinkWriteHandle implements writeHandle for *uplink.Uploads.
type uplinkWriteHandle uplink.Upload

// newUplinkWriteHandle constructs an *uplinkWriteHandle from an *uplink.Upload.
func newUplinkWriteHandle(dl *uplink.Upload) *uplinkWriteHandle {
	return (*uplinkWriteHandle)(dl)
}

func (u *uplinkWriteHandle) raw() *uplink.Upload {
	return (*uplink.Upload)(u)
}

func (u *uplinkWriteHandle) Write(p []byte) (int, error) { return u.raw().Write(p) }
func (u *uplinkWriteHandle) Commit() error               { return u.raw().Commit() }
func (u *uplinkWriteHandle) Abort() error                { return u.raw().Abort() }

// osWriteHandle implements writeHandle for *os.Files.
type osWriteHandle struct {
	fh   *os.File
	done bool
}

// newOSWriteHandle constructs an *osWriteHandle from an *os.File.
func newOSWriteHandle(fh *os.File) *osWriteHandle {
	return &osWriteHandle{fh: fh}
}

func (o *osWriteHandle) Write(p []byte) (int, error) { return o.fh.Write(p) }

func (o *osWriteHandle) Commit() error {
	if o.done {
		return nil
	}
	o.done = true

	return o.fh.Close()
}

func (o *osWriteHandle) Abort() error {
	if o.done {
		return nil
	}
	o.done = true

	return errs.Combine(
		o.fh.Close(),
		os.Remove(o.fh.Name()),
	)
}

// genericWriteHandle implements writeHandle for an io.Writer.
type genericWriteHandle struct{ w io.Writer }

// newGenericWriteHandle constructs a *genericWriteHandle from an io.Writer.
func newGenericWriteHandle(w io.Writer) *genericWriteHandle {
	return &genericWriteHandle{w: w}
}

func (g *genericWriteHandle) Write(p []byte) (int, error) { return g.w.Write(p) }
func (g *genericWriteHandle) Commit() error               { return nil }
func (g *genericWriteHandle) Abort() error                { return nil }

//
// object iteration
//

// ObjectIterator is an interface type for iterating over objectInfo values.
type ObjectIterator interface {
	Next() bool
	Err() error
	Item() ObjectInfo
}

// filteredObjectIterator removes any iteration entries that do not begin with the filter.
// all entries must begin with the trim string which is removed before checking for the
// filter.
type filteredObjectIterator struct {
	trim   string
	filter string
	iter   ObjectIterator
}

func (f *filteredObjectIterator) Next() bool {
	for {
		if !f.iter.Next() {
			return false
		}
		key := f.iter.Item().Loc.Key()
		if !strings.HasPrefix(key, f.trim) {
			return false
		}
		if strings.HasPrefix(key, f.filter) {
			return true
		}
	}
}

func (f *filteredObjectIterator) Err() error { return f.iter.Err() }

func (f *filteredObjectIterator) Item() ObjectInfo {
	item := f.iter.Item()
	item.Loc = item.Loc.RemoveKeyPrefix(f.trim)
	return item
}

// emptyObjectIterator is an objectIterator that has no objects.
type emptyObjectIterator struct{}

func (emptyObjectIterator) Next() bool       { return false }
func (emptyObjectIterator) Err() error       { return nil }
func (emptyObjectIterator) Item() ObjectInfo { return ObjectInfo{} }
