// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build linux darwin netbsd freebsd openbsd

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

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

	nfs := pathfs.NewPathNodeFs(newStorjFS(ctx, store), nil)
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

type storjFS struct {
	ctx          context.Context
	store        objects.Store
	createdFiles map[string]*storjFile
	nodeFS       *pathfs.PathNodeFs
	pathfs.FileSystem
}

func newStorjFS(ctx context.Context, store objects.Store) *storjFS {
	return &storjFS{
		ctx:          ctx,
		store:        store,
		createdFiles: make(map[string]*storjFile),
		FileSystem:   pathfs.NewDefaultFileSystem(),
	}
}

func (sf *storjFS) OnMount(nodeFS *pathfs.PathNodeFs) {
	sf.nodeFS = nodeFS
}

func (sf *storjFS) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	zap.S().Debug("GetAttr: ", name)

	if name == "" {
		return &fuse.Attr{Mode: fuse.S_IFDIR | 0755}, fuse.OK
	}

	// special case for just created files e.g. while coping into directory
	createdFile, ok := sf.createdFiles[name]
	if ok {
		attr := &fuse.Attr{}
		status := createdFile.GetAttr(attr)
		return attr, status
	}

	node := sf.nodeFS.Node(name)
	if node != nil && node.IsDir() {
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

func (sf *storjFS) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	zap.S().Debug("OpenDir: ", name)

	objects, err := sf.listObjects(sf.ctx, name, false)
	if err != nil {
		return nil, fuse.EIO
	}

	var entries []fuse.DirEntry
	for _, object := range objects {
		path := object.Path

		mode := fuse.S_IFREG
		if object.IsPrefix {
			path = strings.TrimSuffix(path, "/")
			mode = fuse.S_IFDIR
		}
		entries = append(entries, fuse.DirEntry{Name: path, Mode: uint32(mode)})
	}

	return entries, fuse.OK
}

func (sf *storjFS) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	zap.S().Debug("Mkdir: ", name)

	reader := bytes.NewReader([]byte{})
	meta := objects.SerializableMeta{ContentType: "application/directory"}
	expTime := time.Time{}

	_, err := sf.store.Put(sf.ctx, name+"/", reader, meta, expTime)
	if err != nil {
		return fuse.EIO
	}

	return fuse.OK
}

func (sf *storjFS) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	zap.S().Debug("Rmdir: ", name)

	objects, err := sf.listObjects(sf.ctx, name, true)
	if err != nil {
		zap.S().Errorf("error during listing: %v", err)
		return fuse.EIO
	}

	for _, object := range objects {
		err := sf.store.Delete(sf.ctx, name+"/"+object.Path)
		if err != nil {
			zap.S().Errorf("error during deleting: %v", err)
			return fuse.EIO
		}
	}

	return fuse.OK
}

func (sf *storjFS) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	zap.S().Debug("Open: ", name)
	return newStorjFile(sf.ctx, name, sf.store, false, sf), fuse.OK
}

func (sf *storjFS) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	zap.S().Debug("Create: ", name)

	return sf.addCreatedFile(name, newStorjFile(sf.ctx, name, sf.store, true, sf)), fuse.OK
}

func (sf *storjFS) addCreatedFile(name string, file *storjFile) *storjFile {
	sf.createdFiles[name] = file
	return file
}

func (sf *storjFS) removeCreatedFile(name string) {
	delete(sf.createdFiles, name)
}

func (sf *storjFS) listObjects(ctx context.Context, name string, recursive bool) (c []objects.ListItem, err error) {
	var entries []objects.ListItem

	startAfter := ""

	for {
		items, more, err := sf.store.List(sf.ctx, name, startAfter, "", recursive, 0, meta.None)
		if err != nil {
			return nil, err
		}

		entries = append(entries, items...)

		if !more {
			break
		}

		startAfter = items[len(items)-1].Path
	}

	return entries, nil
}

func (sf *storjFS) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	zap.S().Debug("Unlink: ", name)

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
	ctx             context.Context
	store           objects.Store
	created         bool
	name            string
	size            uint64
	mtime           uint64
	reader          io.ReadCloser
	writer          *io.PipeWriter
	predictedOffset int64
	FS              *storjFS

	nodefs.File
}

func newStorjFile(ctx context.Context, name string, store objects.Store, created bool, FS *storjFS) *storjFile {
	return &storjFile{
		name:    name,
		ctx:     ctx,
		store:   store,
		mtime:   uint64(time.Now().Unix()),
		created: created,
		FS:      FS,
		File:    nodefs.NewDefaultFile(),
	}
}

func (f *storjFile) GetAttr(attr *fuse.Attr) fuse.Status {
	zap.S().Debug("GetAttr file: ", f.name)

	if f.created {
		attr.Mode = fuse.S_IFREG | 0644
		if f.size != 0 {
			attr.Size = f.size
		}
		attr.Mtime = f.mtime
		return fuse.OK
	}
	return fuse.ENOSYS
}

func (f *storjFile) Read(buf []byte, off int64) (res fuse.ReadResult, code fuse.Status) {
	// Detect if offset was moved manually (e.g. stream rev/fwd)
	if off != f.predictedOffset {
		f.closeReader()
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

func (f *storjFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	writer, err := f.getWriter(off)
	if err != nil {
		return 0, fuse.EIO
	}

	written, err := writer.Write(data)
	if err != nil {
		return 0, fuse.EIO
	}

	f.size += uint64(written)
	return uint32(written), fuse.OK
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

func (f *storjFile) getWriter(off int64) (*io.PipeWriter, error) {
	if off == 0 {
		f.size = 0
		f.closeWriter()

		var reader *io.PipeReader
		reader, f.writer = io.Pipe()
		go func() {
			zap.S().Debug("Starts writting: ", f.name)

			defer func() {
				utils.LogClose(reader)
				if f.created {
					f.FS.removeCreatedFile(f.name)
				}
			}()

			meta := objects.SerializableMeta{}
			expTime := time.Time{}

			m, err := f.store.Put(f.ctx, f.name, reader, meta, expTime)
			if err != nil {
				zap.S().Errorf("error during writting: %v", err)
			}

			f.size = uint64(m.Size)

			zap.S().Debug("Stops writting: ", f.name)
		}()
	}
	return f.writer, nil
}

func (f *storjFile) Flush() fuse.Status {
	zap.S().Debug("Flush: ", f.name)

	f.closeReader()
	f.closeWriter()
	return fuse.OK
}

func (f *storjFile) closeReader() {
	if f.reader != nil {
		utils.LogClose(f.reader)
		f.reader = nil
	}
}

func (f *storjFile) closeWriter() {
	if f.writer != nil {
		utils.LogClose(f.writer)
		f.writer = nil
	}
}
