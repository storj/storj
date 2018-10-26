// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build linux darwin netbsd freebsd openbsd

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storage/meta"
	"storj.io/storj/pkg/storage/objects"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mount",
		Short: "Mount a bucket",
		RunE:  mountBucket,
	}, CLICmd)
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

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}
	if src.IsLocal() {
		return fmt.Errorf("No bucket specified. Use format sj://bucket/")
	}

	store, err := bs.GetObjectStore(ctx, src.Bucket())
	if err != nil {
		return err
	}

	nfs := pathfs.NewPathNodeFs(&storjFs{FileSystem: pathfs.NewDefaultFileSystem(), ctx: ctx, store: store}, nil)
	conn := nodefs.NewFileSystemConnector(nfs.Root(), nil)

	// workaround to avoid async (unordered) reading
	mountOpts := fuse.MountOptions{MaxBackground: 1}
	server, err := fuse.NewServer(conn.RawFS(), args[1], &mountOpts)
	if err != nil {
		return fmt.Errorf("Mount failed: %v", err)
	}

	// detect control-c and unmount
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		for range sig {
			if err := server.Unmount(); err != nil {
				fmt.Printf("Unmount failed: %v", err)
			}
		}
	}()

	server.Serve()
	return nil
}

type storjFs struct {
	ctx   context.Context
	store objects.Store
	pathfs.FileSystem
}

func (sf *storjFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	if name == "" {
		return &fuse.Attr{Mode: fuse.S_IFDIR | 0755}, fuse.OK
	}

	metadata, err := sf.store.Meta(sf.ctx, name)
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return nil, fuse.EIO
	}

	// file not found so maybe it's a prefix/directory
	if err != nil {
		items, _, err := sf.store.List(sf.ctx, name, "", "", false, 1, meta.None)
		if err != nil && !storage.ErrKeyNotFound.Has(err) {
			return nil, fuse.EIO
		}

		// when at least one element has this prefix then it's directory
		if len(items) == 1 {
			return &fuse.Attr{Mode: fuse.S_IFDIR | 0755}, fuse.OK
		}

		return nil, fuse.ENOENT
	}

	return &fuse.Attr{
		Owner: *fuse.CurrentOwner(),
		Mode:  fuse.S_IFREG | 0644,
		Size:  uint64(metadata.Size),
		Mtime: uint64(metadata.Modified.Unix()),
	}, fuse.OK
}

func (sf *storjFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	entries, err := sf.listFiles(sf.ctx, name, sf.store)
	if err != nil {
		return nil, fuse.EIO
	}

	return entries, fuse.OK
}

func (sf *storjFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return &storjFile{
		name:  name,
		ctx:   sf.ctx,
		store: sf.store,
		File:  nodefs.NewDefaultFile(),
	}, fuse.OK
}

func (sf *storjFs) listFiles(ctx context.Context, name string, store objects.Store) (c []fuse.DirEntry, err error) {
	var entries []fuse.DirEntry

	startAfter := ""

	for {
		items, more, err := store.List(ctx, name, startAfter, "", false, 0, meta.Modified)
		if err != nil {
			return nil, err
		}

		for _, object := range items {
			path := object.Path

			mode := fuse.S_IFREG
			if object.IsPrefix {
				mode = fuse.S_IFDIR
			}
			entries = append(entries, fuse.DirEntry{Name: path, Mode: uint32(mode)})
		}

		if !more {
			break
		}

		startAfter = items[len(items)-1].Path
	}

	return entries, nil
}

func (sf *storjFs) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	err := sf.store.Delete(sf.ctx, name)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return fuse.ENOENT
		}
		return fuse.EIO
	}

	return fuse.OK
}

type storjFile struct {
	name            string
	ctx             context.Context
	store           objects.Store
	reader          io.ReadCloser
	predictedOffset int64
	nodefs.File
}

func (f *storjFile) Read(buf []byte, off int64) (res fuse.ReadResult, code fuse.Status) {
	// Detect if offset was moved manually (e.g. stream rev/fwd)
	if off != f.predictedOffset {
		f.close()
	}

	reader, err := f.getReader(off)
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return nil, fuse.ENOENT
		}
		return nil, fuse.EIO
	}

	n, err := io.ReadFull(reader, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fuse.EIO
	}

	f.predictedOffset = off + int64(n)

	return fuse.ReadResultData(buf[:n]), fuse.OK
}

func (f *storjFile) getReader(off int64) (io.ReadCloser, error) {
	if f.reader == nil {
		ranger, _, err := f.store.Get(f.ctx, f.name)
		if err != nil {
			return nil, err
		}

		f.reader, err = ranger.Range(f.ctx, off, ranger.Size()-off)
		if err != nil {
			return nil, err
		}
	}
	return f.reader, nil
}

func (f *storjFile) Flush() fuse.Status {
	f.close()
	return fuse.OK
}

func (f *storjFile) close() {
	if f.reader != nil {
		utils.LogClose(f.reader)
		f.reader = nil
	}
}
