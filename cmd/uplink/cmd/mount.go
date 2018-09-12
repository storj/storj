// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/process"

	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/utils"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mount",
		Short: "Mount a bucket",
		RunE:  mountBucket,
	})
}

func mountBucket(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("No bucket specified for mounting")
	}
	if len(args) == 1 {
		return fmt.Errorf("No destination specified")
	}

	ctx := process.Ctx(cmd)

	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	u, err := utils.ParseURL(args[0])
	if err != nil {
		return err
	}
	if u.Host == "" {
		return fmt.Errorf("No bucket specified. Please use format sj://bucket/")
	}

	store, err := bs.GetObjectStore(ctx, u.Host)
	if err != nil {
		return err
	}

	nfs := pathfs.NewPathNodeFs(&storjFs{FileSystem: pathfs.NewDefaultFileSystem(), ctx: ctx, store: store}, nil)
	server, _, err := nodefs.MountRoot(args[1], nfs.Root(), nil)
	if err != nil {
		return fmt.Errorf("Mount fail: %v", err)
	}
	server.Serve()
	return nil
}

type storjFs struct {
	lock  sync.RWMutex
	ctx   context.Context
	store objects.Store
	items map[string]objects.ListItem
	pathfs.FileSystem
}

func (sf *storjFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	if name == "" {
		return &fuse.Attr{Mode: fuse.S_IFDIR | 0755}, fuse.OK
	}

	sf.lock.RLock()
	defer sf.lock.RUnlock()

	item, has := sf.items[name]

	if has {
		return &fuse.Attr{
			Owner: *fuse.CurrentOwner(),
			Mode:  fuse.S_IFREG | 0644,
			Size:  uint64(item.Meta.Size),
			Mtime: uint64(item.Meta.Modified.Unix()),
		}, fuse.OK
	}

	return nil, fuse.ENOENT
}

func (sf *storjFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	if name == "" {
		err := sf.loadFiles(sf.ctx, sf.store)
		if err != nil {
			fmt.Print(err)
			return nil, fuse.EIO
		}

		entries := make([]fuse.DirEntry, len(sf.items))
		for k := range sf.items {
			var d fuse.DirEntry
			d.Name = k
			d.Mode = fuse.S_IFLNK
			entries = append(entries, fuse.DirEntry(d))
		}

		return entries, fuse.OK
	}
	return nil, fuse.ENOENT
}

func (sf *storjFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return &storjFile{
		ctx:   sf.ctx,
		name:  name,
		store: sf.store,
		File:  nodefs.NewDefaultFile(),
	}, fuse.OK
}

func (sf *storjFs) loadFiles(ctx context.Context, store objects.Store) (err error) {
	sf.lock.RLock()
	defer sf.lock.RUnlock()

	sf.items = make(map[string]objects.ListItem)

	startAfter := paths.New("")

	for {
		items, more, err := store.List(ctx, paths.New(""), startAfter, nil, true, 0, meta.Modified|meta.Size)
		if err != nil {
			return err
		}

		for _, object := range items {
			path := object.Path.String()
			sf.items[path] = object
		}

		if !more {
			break
		}

		startAfter = items[len(items)-1].Path
	}

	return nil
}

type storjFile struct {
	name  string
	ctx   context.Context
	store objects.Store
	nodefs.File
}

func (f *storjFile) Read(buf []byte, off int64) (res fuse.ReadResult, code fuse.Status) {
	rr, _, err := f.store.Get(f.ctx, paths.New(f.name))
	if err != nil {
		fmt.Print(err)
		return nil, fuse.EIO
	}
	defer utils.LogClose(rr)

	length := int64(len(buf))
	if rr.Size()-off < length {
		length = rr.Size() - off
	}
	r, err := rr.Range(f.ctx, off, length)
	if err != nil {
		fmt.Print(err)
		return nil, fuse.EIO
	}
	defer utils.LogClose(r)

	bytesBuf := new(bytes.Buffer)
	_, err = bytesBuf.ReadFrom(r)

	if err != nil {
		fmt.Print(err)
		return nil, fuse.EIO
	}

	return fuse.ReadResultData(bytesBuf.Bytes()), fuse.OK
}
