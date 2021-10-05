// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"bytes"
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/storj/cmd/uplinkng/ulloc"
)

//
// ulfs.Filesystem
//

type testFilesystem struct {
	stdin   string
	created int64
	files   map[ulloc.Location]memFileData
	pending map[ulloc.Location][]*memWriteHandle
	locals  map[string]bool // true means path is a directory
	buckets map[string]struct{}

	mu sync.Mutex
}

func newTestFilesystem() *testFilesystem {
	return &testFilesystem{
		files:   make(map[ulloc.Location]memFileData),
		pending: make(map[ulloc.Location][]*memWriteHandle),
		locals:  make(map[string]bool),
		buckets: make(map[string]struct{}),
	}
}

type memFileData struct {
	contents string
	created  int64
}

func (tfs *testFilesystem) ensureBucket(name string) {
	tfs.buckets[name] = struct{}{}
}

func (tfs *testFilesystem) Files() (files []File) {
	for loc, mf := range tfs.files {
		files = append(files, File{
			Loc:      loc.String(),
			Contents: mf.contents,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].less(files[j]) })
	return files
}

func (tfs *testFilesystem) Pending() (files []File) {
	for loc, mh := range tfs.pending {
		for _, h := range mh {
			files = append(files, File{
				Loc:      loc.String(),
				Contents: h.buf.String(),
			})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].less(files[j]) })
	return files
}

func (tfs *testFilesystem) Close() error {
	return nil
}

func (tfs *testFilesystem) Open(ctx clingy.Context, loc ulloc.Location, opts *ulfs.OpenOptions) (_ ulfs.ReadHandle, err error) {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	if loc.Std() {
		return &byteReadHandle{Buffer: bytes.NewBufferString("-")}, nil
	}

	mf, ok := tfs.files[loc]
	if !ok {
		return nil, errs.New("file does not exist %q", loc)
	}

	if opts != nil {
		return &byteReadHandle{Buffer: bytes.NewBufferString(mf.contents[opts.Offset:(opts.Offset + opts.Length)])}, nil
	}

	return &byteReadHandle{Buffer: bytes.NewBufferString(mf.contents)}, nil
}

func (tfs *testFilesystem) Create(ctx clingy.Context, loc ulloc.Location) (_ ulfs.WriteHandle, err error) {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	if loc.Std() {
		return new(discardWriteHandle), nil
	}

	if bucket, _, ok := loc.RemoteParts(); ok {
		if _, ok := tfs.buckets[bucket]; !ok {
			return nil, errs.New("bucket %q does not exist", bucket)
		}
	}

	if path, ok := loc.LocalParts(); ok {
		if loc.Directoryish() || tfs.isLocalDir(ctx, loc) {
			return nil, errs.New("unable to open file for writing: %q", loc)
		}
		dir := ulloc.CleanPath(filepath.Dir(path))
		if err := tfs.mkdirAll(ctx, dir); err != nil {
			return nil, err
		}
	}

	tfs.created++
	wh := &memWriteHandle{
		buf: bytes.NewBuffer(nil),
		loc: loc,
		tfs: tfs,
		cre: tfs.created,
	}

	if loc.Remote() {
		tfs.pending[loc] = append(tfs.pending[loc], wh)
	}

	return wh, nil
}

func (tfs *testFilesystem) Move(ctx clingy.Context, source, dest ulloc.Location) error {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	mf, ok := tfs.files[source]
	if !ok {
		return errs.New("file does not exist %q", source)
	}
	delete(tfs.files, source)
	tfs.files[dest] = mf
	return nil
}

func (tfs *testFilesystem) Remove(ctx context.Context, loc ulloc.Location, opts *ulfs.RemoveOptions) error {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	if opts == nil || !opts.Pending {
		delete(tfs.files, loc)
	} else {
		// TODO: Remove needs an API that understands that multiple pending files may exist
		delete(tfs.pending, loc)
	}
	return nil
}

func (tfs *testFilesystem) List(ctx context.Context, prefix ulloc.Location, opts *ulfs.ListOptions) (ulfs.ObjectIterator, error) {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	if opts != nil && opts.Pending {
		return tfs.listPending(ctx, prefix, opts)
	}

	prefixDir := prefix.AsDirectoryish()

	var infos []ulfs.ObjectInfo
	for loc, mf := range tfs.files {
		if loc.HasPrefix(prefixDir) || loc == prefix {
			infos = append(infos, ulfs.ObjectInfo{
				Loc:     loc,
				Created: time.Unix(mf.created, 0),
			})
		}
	}

	sort.Sort(objectInfos(infos))

	if opts == nil || !opts.Recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}, nil
}

func (tfs *testFilesystem) listPending(ctx context.Context, prefix ulloc.Location, opts *ulfs.ListOptions) (ulfs.ObjectIterator, error) {
	if prefix.Local() {
		return &objectInfoIterator{}, nil
	}

	prefixDir := prefix.AsDirectoryish()

	var infos []ulfs.ObjectInfo
	for loc, whs := range tfs.pending {
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

	return &objectInfoIterator{infos: infos}, nil
}

func (tfs *testFilesystem) IsLocalDir(ctx context.Context, loc ulloc.Location) (local bool) {
	tfs.mu.Lock()
	defer tfs.mu.Unlock()

	return tfs.isLocalDir(ctx, loc)
}

func (tfs *testFilesystem) isLocalDir(ctx context.Context, loc ulloc.Location) (local bool) {
	path, ok := loc.LocalParts()
	return ok && (ulloc.CleanPath(path) == "." || tfs.locals[path])
}

func (tfs *testFilesystem) Stat(ctx context.Context, loc ulloc.Location) (*ulfs.ObjectInfo, error) {
	if loc.Std() {
		return nil, errs.New("unable to stat loc %q", loc.Loc())
	}

	mf, ok := tfs.files[loc]
	if !ok {
		return nil, errs.New("file does not exist: %q", loc.Loc())
	}

	return &ulfs.ObjectInfo{
		Loc:           loc,
		Created:       time.Unix(mf.created, 0),
		ContentLength: int64(len(mf.contents)),
	}, nil
}

func (tfs *testFilesystem) mkdirAll(ctx context.Context, dir string) error {
	i := 0
	for i < len(dir) {
		slash := strings.Index(dir[i:], "/")
		if slash == -1 {
			break
		}
		if err := tfs.mkdir(ctx, dir[:i+slash]); err != nil {
			return err
		}
		i += slash + 1
	}
	if len(dir) > 0 {
		return tfs.mkdir(ctx, dir)
	}
	return nil
}

func (tfs *testFilesystem) mkdir(ctx context.Context, dir string) error {
	if isDir, ok := tfs.locals[dir]; ok && !isDir {
		return errs.New("cannot create directory: %q is a file", dir)
	}
	tfs.locals[dir] = true
	return nil
}

//
// ulfs.ReadHandle
//

type byteReadHandle struct {
	*bytes.Buffer
}

func (b *byteReadHandle) Close() error          { return nil }
func (b *byteReadHandle) Info() ulfs.ObjectInfo { return ulfs.ObjectInfo{} }

//
// ulfs.WriteHandle
//

type memWriteHandle struct {
	buf  *bytes.Buffer
	loc  ulloc.Location
	tfs  *testFilesystem
	cre  int64
	done bool
}

func (b *memWriteHandle) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

func (b *memWriteHandle) Commit() error {
	b.tfs.mu.Lock()
	defer b.tfs.mu.Unlock()

	if err := b.close(); err != nil {
		return err
	}

	if path, ok := b.loc.LocalParts(); ok {
		b.tfs.locals[path] = false
	}

	b.tfs.files[b.loc] = memFileData{
		contents: b.buf.String(),
		created:  b.cre,
	}
	return nil
}

func (b *memWriteHandle) Abort() error {
	b.tfs.mu.Lock()
	defer b.tfs.mu.Unlock()

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

	handles := b.tfs.pending[b.loc]
	for i, v := range handles {
		if v == b {
			handles = append(handles[:i], handles[i+1:]...)
			break
		}
	}

	if len(handles) > 0 {
		b.tfs.pending[b.loc] = handles
	} else {
		delete(b.tfs.pending, b.loc)
	}

	return nil
}

type discardWriteHandle struct{}

func (discardWriteHandle) Write(p []byte) (int, error) { return len(p), nil }
func (discardWriteHandle) Commit() error               { return nil }
func (discardWriteHandle) Abort() error                { return nil }

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
