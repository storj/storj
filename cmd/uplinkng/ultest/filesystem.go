// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"bytes"
	"context"
	"sort"
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
	ops     []Operation
	created int64
	files   map[ulloc.Location]byteFileData
	pending map[ulloc.Location][]*byteWriteHandle
	buckets map[string]struct{}
}

func newTestFilesystem() *testFilesystem {
	return &testFilesystem{
		files:   make(map[ulloc.Location]byteFileData),
		pending: make(map[ulloc.Location][]*byteWriteHandle),
		buckets: make(map[string]struct{}),
	}
}

type byteFileData struct {
	data    []byte
	created int64
}

func (tfs *testFilesystem) ensureBucket(name string) {
	tfs.buckets[name] = struct{}{}
}

func (tfs *testFilesystem) Close() error {
	return nil
}

func (tfs *testFilesystem) Open(ctx clingy.Context, loc ulloc.Location) (_ ulfs.ReadHandle, err error) {
	defer func() { tfs.ops = append(tfs.ops, newOp("open", loc, err)) }()

	bf, ok := tfs.files[loc]
	if !ok {
		return nil, errs.New("file does not exist")
	}
	return &byteReadHandle{Buffer: bytes.NewBuffer(bf.data)}, nil
}

func (tfs *testFilesystem) Create(ctx clingy.Context, loc ulloc.Location) (_ ulfs.WriteHandle, err error) {
	defer func() { tfs.ops = append(tfs.ops, newOp("create", loc, err)) }()

	if bucket, _, ok := loc.RemoteParts(); ok {
		if _, ok := tfs.buckets[bucket]; !ok {
			return nil, errs.New("bucket %q does not exist", bucket)
		}
	}

	tfs.created++
	wh := &byteWriteHandle{
		buf: bytes.NewBuffer(nil),
		loc: loc,
		tfs: tfs,
		cre: tfs.created,
	}

	tfs.pending[loc] = append(tfs.pending[loc], wh)

	return wh, nil
}

func (tfs *testFilesystem) ListObjects(ctx context.Context, prefix ulloc.Location, recursive bool) (ulfs.ObjectIterator, error) {
	var infos []ulfs.ObjectInfo
	for loc, bf := range tfs.files {
		if loc.HasPrefix(prefix) {
			infos = append(infos, ulfs.ObjectInfo{
				Loc:     loc,
				Created: time.Unix(bf.created, 0),
			})
		}
	}

	sort.Sort(objectInfos(infos))

	if !recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}, nil
}

func (tfs *testFilesystem) ListUploads(ctx context.Context, prefix ulloc.Location, recursive bool) (ulfs.ObjectIterator, error) {
	var infos []ulfs.ObjectInfo
	for loc, whs := range tfs.pending {
		if loc.HasPrefix(prefix) {
			for _, wh := range whs {
				infos = append(infos, ulfs.ObjectInfo{
					Loc:     loc,
					Created: time.Unix(wh.cre, 0),
				})
			}
		}
	}

	sort.Sort(objectInfos(infos))

	if !recursive {
		infos = collapseObjectInfos(prefix, infos)
	}

	return &objectInfoIterator{infos: infos}, nil
}

func (tfs *testFilesystem) IsLocalDir(ctx context.Context, loc ulloc.Location) bool {
	// TODO: implement this

	return false
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

type byteWriteHandle struct {
	buf  *bytes.Buffer
	loc  ulloc.Location
	tfs  *testFilesystem
	cre  int64
	done bool
}

func (b *byteWriteHandle) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

func (b *byteWriteHandle) Commit() error {
	if err := b.close(); err != nil {
		return err
	}

	b.tfs.ops = append(b.tfs.ops, newOp("commit", b.loc, nil))
	b.tfs.files[b.loc] = byteFileData{
		data:    b.buf.Bytes(),
		created: b.cre,
	}
	return nil
}

func (b *byteWriteHandle) Abort() error {
	if err := b.close(); err != nil {
		return err
	}

	b.tfs.ops = append(b.tfs.ops, newOp("append", b.loc, nil))
	return nil
}

func (b *byteWriteHandle) close() error {
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

		oi.Loc = oi.Loc.SetKey(first)

		infos[j] = oi
		j++
	}

	return infos[:j]
}
