// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"io"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
	"storj.io/storj/pkg/storage/streams"
)

var mon = monkit.Package()

// Store for objects
type Store interface {
	GetObjectMeta(ctx context.Context, path paths.Path) (meta storage.Meta,
		err error)
	GetObject(ctx context.Context, path paths.Path) (rr ranger.RangeCloser,
		meta storage.Meta, err error)
	PutObject(ctx context.Context, path paths.Path, data io.Reader,
		metadata []byte, expiration time.Time) (meta storage.Meta, err error)
	DeleteObject(ctx context.Context, path paths.Path) (err error)
	ListObjects(ctx context.Context, root, startAfter, endBefore paths.Path,
		recursive bool, limit int) (result []paths.Path, more bool, err error)
	GetXAttr(ctx context.Context, path paths.Path, xattr string) (
		rr ranger.RangeCloser, meta storage.Meta, err error)
	SetXAttr(ctx context.Context, path paths.Path, xattr string,
		data io.Reader, metadata []byte) (meta storage.Meta, err error)
	DeleteXAttr(ctx context.Context, path paths.Path, xattr string) (err error)
	ListXAttrs(ctx context.Context, path paths.Path, startAfter,
		endBefore string, limit int) (xattrs []string, more bool, err error)
}

type objStore struct {
	s streams.Store
}

// NewStore for objects
func NewStore(store streams.Store) Store {
	return &objStore{s: store}
}

func (o *objStore) PutObject(ctx context.Context, path paths.Path,
	data io.Reader, metadata []byte, expiration time.Time) (
	meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Put(ctx, objPath, data, metadata, expiration)
}

func (o *objStore) GetObject(ctx context.Context, path paths.Path) (
	rr ranger.RangeCloser, meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Get(ctx, objPath)
}

func (o *objStore) GetObjectMeta(ctx context.Context, path paths.Path) (
	meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Meta(ctx, objPath)
}

func (o *objStore) DeleteObject(ctx context.Context, path paths.Path) (
	err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	return o.s.Delete(ctx, objPath)
}

func (o *objStore) ListObjects(ctx context.Context, root, startAfter,
	endBefore paths.Path, recursive bool, limit int) (
	result []paths.Path, more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	rootObjPath := getDefautStreamPath(root)
	return o.s.List(ctx, rootObjPath, startAfter, endBefore, recursive, limit)
}

func (o *objStore) SetXAttr(ctx context.Context, path paths.Path, xattr string,
	data io.Reader, metadata []byte) (meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	objPath := getDefautStreamPath(path)
	objMeta, err := o.s.Meta(ctx, objPath)
	if err != nil {
		return storage.Meta{}, err
	}
	xattrPath := getNamedStreamPath(path, xattr)
	return o.s.Put(ctx, xattrPath, data, metadata, objMeta.Expiration)
}

func (o *objStore) GetXAttr(ctx context.Context, path paths.Path,
	xattr string) (rr ranger.RangeCloser, meta storage.Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	xattrPath := getNamedStreamPath(path, xattr)
	return o.s.Get(ctx, xattrPath)
}

func (o *objStore) DeleteXAttr(ctx context.Context, path paths.Path,
	xattr string) (err error) {
	defer mon.Task()(&ctx)(&err)
	xattrPath := getNamedStreamPath(path, xattr)
	return o.s.Delete(ctx, xattrPath)
}

func (o *objStore) ListXAttrs(ctx context.Context, path paths.Path, startAfter,
	endBefore string, limit int) (xattrs []string, more bool, err error) {
	defer mon.Task()(&ctx)(&err)
	root := getNamedStreamsRoot(path)
	paths, more, err := o.s.List(ctx, root, paths.New(startAfter),
		paths.New(endBefore), false, limit)
	if err != nil {
		return []string{}, false, err
	}
	xattrs = make([]string, len(paths))
	for i, p := range paths {
		xattrs[i] = p[len(p)-1]
	}
	return xattrs, more, nil
}

func getDefautStreamPath(path paths.Path) paths.Path {
	return path.Prepend("object")
}

func getNamedStreamsRoot(path paths.Path) paths.Path {
	return path.Prepend("xattr")
}

func getNamedStreamPath(path paths.Path, xattr string) paths.Path {
	return getNamedStreamsRoot(path).Append(xattr)
}
