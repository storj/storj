// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
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
	files   map[ulloc.Location][]memFileData
	pending map[ulloc.Location][]*memWriteHandle
	buckets map[string]struct{}

	mu sync.Mutex
}

func newRemoteFilesystem() *remoteFilesystem {
	return &remoteFilesystem{
		files:   make(map[ulloc.Location][]memFileData),
		pending: make(map[ulloc.Location][]*memWriteHandle),
		buckets: make(map[string]struct{}),
	}
}

type memFileData struct {
	version          int64
	contents         string
	created          int64
	expires          time.Time
	metadata         map[string]string
	isDeleteMarker   bool
	governanceLocked bool
}

func (mf memFileData) expired() bool {
	return mf.expires != time.Time{} && mf.expires.Before(time.Now())
}

func (rfs *remoteFilesystem) ensureBucket(name string) {
	rfs.buckets[name] = struct{}{}
}

func (rfs *remoteFilesystem) Files() (files []File) {
	for loc, fileVersions := range rfs.files {
		for _, mf := range fileVersions {
			if mf.expired() {
				continue
			}
			files = append(files, File{
				Loc:      loc.String(),
				Version:  mf.version,
				Contents: mf.contents,
				Metadata: mf.metadata,
			})
		}
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

	files := rfs.files[loc]
	if len(files) == 0 {
		return nil, errs.New("file does not exist %q", loc)
	}

	return newMultiReadHandle(files[len(files)-1].contents), nil
}

type createOpts struct {
	ulfs.CreateOptions
	versioned        bool
	governanceLocked bool
}

func (rfs *remoteFilesystem) Create(ctx context.Context, bucket, key string, opts *ulfs.CreateOptions) (ulfs.MultiWriteHandle, error) {
	internalOpts := createOpts{}
	if opts != nil {
		internalOpts.CreateOptions = *opts
	}
	return rfs.create(ctx, bucket, key, internalOpts)
}

func (rfs *remoteFilesystem) create(ctx context.Context, bucket, key string, opts createOpts) (_ ulfs.MultiWriteHandle, err error) {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	loc := ulloc.NewRemote(bucket, key)

	if _, ok := rfs.buckets[bucket]; !ok {
		return nil, errs.New("bucket %q does not exist", bucket)
	}

	rfs.created++
	wh := &memWriteHandle{
		loc:              loc,
		rfs:              rfs,
		cre:              rfs.created,
		expires:          opts.Expires,
		metadata:         opts.Metadata,
		versioned:        opts.versioned,
		governanceLocked: opts.governanceLocked,
	}

	rfs.pending[loc] = append(rfs.pending[loc], wh)

	return ulfs.NewGenericMultiWriteHandle(wh), nil
}

func (rfs *remoteFilesystem) createDeleteMarker(ctx context.Context, bucket, key string) {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	loc := ulloc.NewRemote(bucket, key)

	var version int64
	files := rfs.files[loc]
	if len(files) > 0 {
		version = files[len(files)-1].version + 1
	}

	rfs.created++
	rfs.files[loc] = append(files, memFileData{
		version:        version,
		isDeleteMarker: true,
		created:        rfs.created,
	})
}

func (rfs *remoteFilesystem) Move(ctx context.Context, oldbucket, oldkey string, newbucket, newkey string) error {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	source := ulloc.NewRemote(oldbucket, oldkey)
	dest := ulloc.NewRemote(newbucket, newkey)

	sourceFiles := rfs.files[source]
	if len(sourceFiles) == 0 {
		return errs.New("file does not exist %q", source)
	}

	file := sourceFiles[len(sourceFiles)-1]
	file.version = 0

	delete(rfs.files, source)

	if rfs.files[dest] != nil {
		rfs.files[dest] = rfs.files[dest][:0]
	}

	rfs.files[dest] = append(rfs.files[dest], file)

	return nil
}

func (rfs *remoteFilesystem) Copy(ctx context.Context, oldbucket, oldkey string, newbucket, newkey string) error {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	source := ulloc.NewRemote(oldbucket, oldkey)
	dest := ulloc.NewRemote(newbucket, newkey)

	sourceFiles := rfs.files[source]
	if len(sourceFiles) == 0 {
		return errs.New("file does not exist %q", source)
	}

	file := sourceFiles[len(sourceFiles)-1]
	file.version = 0

	if rfs.files[dest] != nil {
		rfs.files[dest] = rfs.files[dest][:0]
	}

	rfs.files[dest] = append(rfs.files[dest], file)

	return nil
}

func (rfs *remoteFilesystem) Remove(ctx context.Context, bucket, key string, opts *ulfs.RemoveOptions) error {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()

	loc := ulloc.NewRemote(bucket, key)

	if opts == nil || !opts.Pending {
		if opts.Version != nil {
			version := int64(binary.BigEndian.Uint64(opts.Version))
			files := rfs.files[loc]
			for i, file := range files {
				if file.version != version {
					continue
				}
				if file.governanceLocked && !opts.BypassGovernanceRetention {
					return errs.New("file is protected by Object Lock settings")
				}

				rfs.files[loc] = append(files[:i], files[i+1:]...)
				if len(rfs.files[loc]) == 0 {
					delete(rfs.files, loc)
				}

				return nil
			}
			return errs.New("file does not exist: %q version %s", loc, hex.EncodeToString(opts.Version))
		}
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
	for loc, files := range rfs.files {
		if !loc.HasPrefix(prefixDir) && loc != prefix {
			continue
		}
		if len(files) == 0 {
			continue
		}

		if opts.AllVersions {
			for _, file := range files {
				if !file.expired() {
					infos = append(infos, memFileDataToObjectInfo(loc, file))
				}
			}
		} else {
			file := files[len(files)-1]
			if !file.expired() && !file.isDeleteMarker {
				infos = append(infos, memFileDataToObjectInfo(loc, file))
			}
		}
	}

	sort.Sort(objectInfos(infos))

	if opts == nil || !opts.Recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}
}

func memFileDataToObjectInfo(loc ulloc.Location, mf memFileData) ulfs.ObjectInfo {
	info := ulfs.ObjectInfo{
		Loc:            loc,
		Version:        make([]byte, 8),
		IsDeleteMarker: mf.isDeleteMarker,
		ContentLength:  int64(len(mf.contents)),
		Created:        time.Unix(mf.created, 0),
		Expires:        mf.expires,
	}
	binary.BigEndian.PutUint64(info.Version, uint64(mf.version))
	return info
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

	files := rfs.files[loc]
	if len(files) == 0 {
		return nil, errs.New("file does not exist: %q", loc.Loc())
	}

	file := files[len(files)-1]
	if file.expired() {
		return nil, errs.New("file does not exist: %q", loc.Loc())
	}

	info := memFileDataToObjectInfo(loc, file)
	return &info, nil
}

//
// ulfs.WriteHandle
//

type memWriteHandle struct {
	buf              []byte
	loc              ulloc.Location
	rfs              *remoteFilesystem
	cre              int64
	expires          time.Time
	metadata         map[string]string
	versioned        bool
	governanceLocked bool
	done             bool
}

func (b *memWriteHandle) WriteAt(p []byte, off int64) (int, error) {
	if b.done {
		return 0, errs.New("write to closed handle")
	}
	end := int64(len(p)) + off
	if grow := end - int64(len(b.buf)); grow > 0 {
		b.buf = append(b.buf, make([]byte, grow)...)
	}
	return copy(b.buf[off:], p), nil
}

func (b *memWriteHandle) Commit() error {
	b.rfs.mu.Lock()
	defer b.rfs.mu.Unlock()

	if err := b.close(); err != nil {
		return err
	}

	file := memFileData{
		contents:         string(b.buf),
		created:          b.cre,
		expires:          b.expires,
		metadata:         b.metadata,
		governanceLocked: b.governanceLocked,
	}

	files := b.rfs.files[b.loc]

	if b.versioned {
		if len(files) > 0 {
			file.version = files[len(files)-1].version + 1
		}
	} else if files != nil {
		files = files[:0]
	}

	b.rfs.files[b.loc] = append(files, file)

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

func (ois objectInfos) Len() int          { return len(ois) }
func (ois objectInfos) Swap(i int, j int) { ois[i], ois[j] = ois[j], ois[i] }
func (ois objectInfos) Less(i int, j int) bool {
	if ois[i].Loc == ois[j].Loc {
		var versionI, versionJ int64
		if ois[i].Version != nil {
			versionI = int64(binary.BigEndian.Uint64(ois[i].Version))
		}
		if ois[j].Version != nil {
			versionJ = int64(binary.BigEndian.Uint64(ois[j].Version))
		}
		return versionI < versionJ
	}
	return ois[i].Loc.Less(ois[j].Loc)
}

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
