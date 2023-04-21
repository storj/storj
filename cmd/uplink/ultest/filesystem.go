// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"bytes"
	"context"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/storj/cmd/uplink/ulloc"
)

//
// ulfs.Filesystem
//

type remoteFilesystem struct {
	created int64
	files   map[ulloc.Location]memFileData
	pending map[ulloc.Location][]*memWriteHandle
	buckets map[string]struct{}

	mu sync.Mutex
}

func newRemoteFilesystem() *remoteFilesystem {
	return &remoteFilesystem{
		files:   make(map[ulloc.Location]memFileData),
		pending: make(map[ulloc.Location][]*memWriteHandle),
		buckets: make(map[string]struct{}),
	}
}

type memFileData struct {
	contents string
	created  int64
	expires  time.Time
	metadata map[string]string
}

func (mf memFileData) expired() bool {
	return mf.expires != time.Time{} && mf.expires.Before(time.Now())
}

func (rfs *remoteFilesystem) ensureBucket(name string) {
	rfs.buckets[name] = struct{}{}
}

func (rfs *remoteFilesystem) Files() (files []File) {
	for loc, mf := range rfs.files {
		if mf.expired() {
			continue
		}
		files = append(files, File{
			Loc:      loc.String(),
			Contents: mf.contents,
			Metadata: mf.metadata,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].less(files[j]) })
	return files
}

func (rfs *remoteFilesystem) Pending() (files []File) {
	for loc, mh := range rfs.pending {
		for _, h := range mh {
			files = append(files, File{
				Loc:      loc.String(),
				Contents: string(h.buf),
				Metadata: h.metadata,
			})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].less(files[j]) })
	return files
}

func (rfs *remoteFilesystem) Close() error {
	return nil
}

type nopClosingGenericReader struct{ io.ReaderAt }

func (n nopClosingGenericReader) Close() error { return nil }

func newMultiReadHandle(contents string) ulfs.MultiReadHandle {
	return ulfs.NewGenericMultiReadHandle(nopClosingGenericReader{
		ReaderAt: bytes.NewReader([]byte(contents)),
	}, ulfs.ObjectInfo{
		ContentLength: int64(len(contents)),
	})
}

func (rfs *remoteFilesystem) Open(ctx context.Context, bucket, key string) (ulfs.MultiReadHandle, error) {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	loc := ulloc.NewRemote(bucket, key)

	mf, ok := rfs.files[loc]
	if !ok {
		return nil, errs.New("file does not exist %q", loc)
	}

	return newMultiReadHandle(mf.contents), nil
}

func (rfs *remoteFilesystem) Create(ctx context.Context, bucket, key string, opts *ulfs.CreateOptions) (_ ulfs.WriteHandle, err error) {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	loc := ulloc.NewRemote(bucket, key)

	if _, ok := rfs.buckets[bucket]; !ok {
		return nil, errs.New("bucket %q does not exist", bucket)
	}

	var metadata map[string]string
	expires := time.Time{}
	if opts != nil {
		expires = opts.Expires
		metadata = opts.Metadata
	}

	rfs.created++
	wh := &memWriteHandle{
		loc:      loc,
		rfs:      rfs,
		cre:      rfs.created,
		expires:  expires,
		metadata: metadata,
	}

	rfs.pending[loc] = append(rfs.pending[loc], wh)

	return wh, nil
}

func (rfs *remoteFilesystem) Move(ctx context.Context, oldbucket, oldkey string, newbucket, newkey string) error {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	source := ulloc.NewRemote(oldbucket, oldkey)
	dest := ulloc.NewRemote(newbucket, newkey)

	mf, ok := rfs.files[source]
	if !ok {
		return errs.New("file does not exist %q", source)
	}
	delete(rfs.files, source)
	rfs.files[dest] = mf
	return nil
}

func (rfs *remoteFilesystem) Copy(ctx context.Context, oldbucket, oldkey string, newbucket, newkey string) error {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	source := ulloc.NewRemote(oldbucket, oldkey)
	dest := ulloc.NewRemote(newbucket, newkey)

	mf, ok := rfs.files[source]
	if !ok {
		return errs.New("file does not exist %q", source)
	}
	rfs.files[dest] = mf
	return nil
}

func (rfs *remoteFilesystem) Remove(ctx context.Context, bucket, key string, opts *ulfs.RemoveOptions) error {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	loc := ulloc.NewRemote(bucket, key)

	if opts == nil || !opts.Pending {
		delete(rfs.files, loc)
	} else {
		// TODO: Remove needs an API that understands that multiple pending files may exist
		delete(rfs.pending, loc)
	}
	return nil
}

func (rfs *remoteFilesystem) List(ctx context.Context, bucket, key string, opts *ulfs.ListOptions) ulfs.ObjectIterator {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	prefix := ulloc.NewRemote(bucket, key)

	if opts != nil && opts.Pending {
		return rfs.listPending(ctx, prefix, opts)
	}

	prefixDir := prefix.AsDirectoryish()

	var infos []ulfs.ObjectInfo
	for loc, mf := range rfs.files {
		if (loc.HasPrefix(prefixDir) || loc == prefix) && !mf.expired() {
			infos = append(infos, ulfs.ObjectInfo{
				Loc:     loc,
				Created: time.Unix(mf.created, 0),
				Expires: mf.expires,
			})
		}
	}

	sort.Sort(objectInfos(infos))

	if opts == nil || !opts.Recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}
}

func (rfs *remoteFilesystem) listPending(ctx context.Context, prefix ulloc.Location, opts *ulfs.ListOptions) ulfs.ObjectIterator {
	prefixDir := prefix.AsDirectoryish()

	var infos []ulfs.ObjectInfo
	for loc, whs := range rfs.pending {
		if loc.HasPrefix(prefixDir) || loc == prefix {
			for _, wh := range whs {
				infos = append(infos, ulfs.ObjectInfo{
					Loc:     loc,
					Created: time.Unix(wh.cre, 0),
				})
			}
		}
	}

	sort.Sort(objectInfos(infos))

	if opts == nil || !opts.Recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}
}

func (rfs *remoteFilesystem) Stat(ctx context.Context, bucket, key string) (*ulfs.ObjectInfo, error) {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	loc := ulloc.NewRemote(bucket, key)

	mf, ok := rfs.files[loc]
	if !ok {
		return nil, errs.New("file does not exist: %q", loc.Loc())
	}

	if mf.expired() {
		return nil, errs.New("file does not exist: %q", loc.Loc())
	}

	return &ulfs.ObjectInfo{
		Loc:           loc,
		Created:       time.Unix(mf.created, 0),
		Expires:       mf.expires,
		ContentLength: int64(len(mf.contents)),
	}, nil
}

//
// ulfs.WriteHandle
//

type memWriteHandle struct {
	buf      []byte
	loc      ulloc.Location
	rfs      *remoteFilesystem
	cre      int64
	expires  time.Time
	metadata map[string]string
	done     bool
}

func (b *memWriteHandle) Write(p []byte) (int, error) {
	if b.done {
		return 0, errs.New("write to closed handle")
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *memWriteHandle) Commit() error {
	b.rfs.mu.Lock()
	defer b.rfs.mu.Unlock()

	if err := b.close(); err != nil {
		return err
	}

	b.rfs.files[b.loc] = memFileData{
		contents: string(b.buf),
		created:  b.cre,
		expires:  b.expires,
		metadata: b.metadata,
	}

	return nil
}

func (b *memWriteHandle) Abort() error {
	b.rfs.mu.Lock()
	defer b.rfs.mu.Unlock()

	if err := b.close(); err != nil {
		return err
	}

	return nil
}

func (b *memWriteHandle) close() error {
	if b.done {
		return errs.New("already done")
	}
	b.done = true

	handles := b.rfs.pending[b.loc]
	for i, v := range handles {
		if v == b {
			handles = append(handles[:i], handles[i+1:]...)
			break
		}
	}

	if len(handles) > 0 {
		b.rfs.pending[b.loc] = handles
	} else {
		delete(b.rfs.pending, b.loc)
	}

	return nil
}

//
// ulfs.ObjectIterator
//

type objectInfoIterator struct {
	infos   []ulfs.ObjectInfo
	current ulfs.ObjectInfo
}

func (li *objectInfoIterator) Next() bool {
	if len(li.infos) == 0 {
		return false
	}
	li.current, li.infos = li.infos[0], li.infos[1:]
	return true
}

func (li *objectInfoIterator) Err() error {
	return nil
}

func (li *objectInfoIterator) Item() ulfs.ObjectInfo {
	return li.current
}

type objectInfos []ulfs.ObjectInfo

func (ois objectInfos) Len() int               { return len(ois) }
func (ois objectInfos) Swap(i int, j int)      { ois[i], ois[j] = ois[j], ois[i] }
func (ois objectInfos) Less(i int, j int) bool { return ois[i].Loc.Less(ois[j].Loc) }

func collapseObjectInfos(prefix ulloc.Location, infos []ulfs.ObjectInfo) []ulfs.ObjectInfo {
	collapsing := false
	current := ""
	j := 0

	for _, oi := range infos {
		first, ok := oi.Loc.ListKeyName(prefix)
		if ok {
			if collapsing && first == current {
				continue
			}

			collapsing = true
			current = first

			oi.IsPrefix = true
		}

		if bucket, _, ok := oi.Loc.RemoteParts(); ok {
			oi.Loc = ulloc.NewRemote(bucket, first)
		} else if _, ok := oi.Loc.LocalParts(); ok {
			oi.Loc = ulloc.NewLocal(first)
		} else {
			panic("invalid object returned from list")
		}

		infos[j] = oi
		j++
	}

	return infos[:j]
}
