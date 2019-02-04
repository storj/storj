// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build linux darwin

package cmd

import (
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
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/stream"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mount",
		Short: "Mount a bucket",
		RunE:  mountBucket,
	}, RootCmd)
}

func mountBucket(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("No bucket specified for mounting")
	}
	if len(args) == 1 {
		return fmt.Errorf("No destination specified")
	}

	ctx := process.Ctx(cmd)

	metainfo, streams, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, nil, cfg.Identity.CertPath); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher: ", err)
	}

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}
	if src.IsLocal() {
		return fmt.Errorf("No bucket specified. Use format sj://bucket/")
	}

	bucket, err := metainfo.GetBucket(ctx, src.Bucket())
	if err != nil {
		return convertError(err, src)
	}

	nfs := pathfs.NewPathNodeFs(newStorjFS(ctx, metainfo, streams, bucket), nil)
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
	metainfo     storj.Metainfo
	streams      streams.Store
	bucket       storj.Bucket
	createdFiles map[string]*storjFile
	nodeFS       *pathfs.PathNodeFs
	pathfs.FileSystem
}

func newStorjFS(ctx context.Context, metainfo storj.Metainfo, streams streams.Store, bucket storj.Bucket) *storjFS {
	return &storjFS{
		ctx:          ctx,
		metainfo:     metainfo,
		streams:      streams,
		bucket:       bucket,
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

	object, err := sf.metainfo.GetObject(sf.ctx, sf.bucket.Name, name)
	if err != nil && !storj.ErrObjectNotFound.Has(err) {
		return nil, fuse.EIO
	}

	// file not found so maybe it's a prefix/directory
	if err != nil {
		list, err := sf.metainfo.ListObjects(sf.ctx, sf.bucket.Name, storj.ListOptions{Direction: storj.After, Prefix: name, Limit: 1})
		if err != nil {
			return nil, fuse.EIO
		}

		// if exactly one element has this prefix then it's directory
		if len(list.Items) == 1 {
			return &fuse.Attr{Mode: fuse.S_IFDIR | 0755}, fuse.OK
		}

		return nil, fuse.ENOENT
	}

	return &fuse.Attr{
		Owner: *fuse.CurrentOwner(),
		Mode:  fuse.S_IFREG | 0644,
		Size:  uint64(object.Size),
		Mtime: uint64(object.Modified.Unix()),
	}, fuse.OK
}

func (sf *storjFS) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	zap.S().Debug("OpenDir: ", name)

	var entries []fuse.DirEntry
	err := sf.listObjects(sf.ctx, name, false, func(items []storj.Object) error {
		for _, item := range items {
			path := item.Path

			mode := fuse.S_IFREG
			if item.IsPrefix {
				path = strings.TrimSuffix(path, "/")
				mode = fuse.S_IFDIR
			}
			entries = append(entries, fuse.DirEntry{Name: path, Mode: uint32(mode)})
		}
		return nil
	})
	if err != nil {
		zap.S().Errorf("error during opening directory: %v", err)
		return nil, fuse.EIO
	}

	return entries, fuse.OK
}

func (sf *storjFS) Mkdir(name string, mode uint32, context *fuse.Context) fuse.Status {
	zap.S().Debug("Mkdir: ", name)

	createInfo := storj.CreateObject{
		ContentType:      "application/directory",
		RedundancyScheme: cfg.GetRedundancyScheme(),
		EncryptionScheme: cfg.GetEncryptionScheme(),
	}
	object, err := sf.metainfo.CreateObject(sf.ctx, sf.bucket.Name, name+"/", &createInfo)
	if err != nil {
		return fuse.EIO
	}

	// TODO: Perhaps we should not create a stream for an empty object.
	// This would be possible after we replace the streams.Store.
	mutableStream, err := object.CreateStream(sf.ctx)
	if err != nil {
		return fuse.EIO
	}

	upload := stream.NewUpload(sf.ctx, mutableStream, sf.streams)
	defer func() {
		if err := upload.Close(); err != nil {
			zap.S().Errorf("Failed to close file: %s", err)
		}
	}()

	_, err = upload.Write(nil)
	if err != nil {
		return fuse.EIO
	}

	return fuse.OK
}

func (sf *storjFS) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	zap.S().Debug("Rmdir: ", name)

	err := sf.listObjects(sf.ctx, name, true, func(items []storj.Object) error {
		for _, item := range items {
			err := sf.metainfo.DeleteObject(sf.ctx, sf.bucket.Name, storj.JoinPaths(name, item.Path))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		zap.S().Errorf("error during removing directory: %v", err)
		return fuse.EIO
	}

	return fuse.OK
}

func (sf *storjFS) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	zap.S().Debug("Open: ", name)
	return newStorjFile(sf.ctx, name, sf.metainfo, sf.streams, sf.bucket, false, sf), fuse.OK
}

func (sf *storjFS) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	zap.S().Debug("Create: ", name)

	return sf.addCreatedFile(name, newStorjFile(sf.ctx, name, sf.metainfo, sf.streams, sf.bucket, true, sf)), fuse.OK
}

func (sf *storjFS) addCreatedFile(name string, file *storjFile) *storjFile {
	sf.createdFiles[name] = file
	return file
}

func (sf *storjFS) removeCreatedFile(name string) {
	delete(sf.createdFiles, name)
}

func (sf *storjFS) listObjects(ctx context.Context, name string, recursive bool, handler func([]storj.Object) error) error {
	startAfter := ""

	for {
		list, err := sf.metainfo.ListObjects(sf.ctx, sf.bucket.Name, storj.ListOptions{
			Direction: storj.After,
			Cursor:    startAfter,
			Prefix:    name,
			Recursive: recursive,
		})
		if err != nil {
			return err
		}

		err = handler(list.Items)
		if err != nil {
			return err
		}

		if !list.More {
			break
		}

		startAfter = list.Items[len(list.Items)-1].Path
	}

	return nil
}

func (sf *storjFS) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	zap.S().Debug("Unlink: ", name)

	err := sf.metainfo.DeleteObject(sf.ctx, sf.bucket.Name, name)
	if err != nil {
		if storj.ErrObjectNotFound.Has(err) {
			return fuse.ENOENT
		}
		return fuse.EIO
	}

	return fuse.OK
}

type storjFile struct {
	ctx             context.Context
	metainfo        storj.Metainfo
	streams         streams.Store
	bucket          storj.Bucket
	created         bool
	name            string
	size            uint64
	mtime           uint64
	reader          io.ReadCloser
	writer          io.WriteCloser
	mutableObject   storj.MutableObject
	predictedOffset int64
	FS              *storjFS

	nodefs.File
}

func newStorjFile(ctx context.Context, name string, metainfo storj.Metainfo, streams streams.Store, bucket storj.Bucket, created bool, FS *storjFS) *storjFile {
	return &storjFile{
		name:     name,
		ctx:      ctx,
		metainfo: metainfo,
		streams:  streams,
		bucket:   bucket,
		mtime:    uint64(time.Now().Unix()),
		created:  created,
		FS:       FS,
		File:     nodefs.NewDefaultFile(),
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
		if storj.ErrObjectNotFound.Has(err) {
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

	zap.S().Debug("Starts writting: ", f.name)

	written, err := writer.Write(data)
	if err != nil {
		return 0, fuse.EIO
	}

	f.size += uint64(written)

	zap.S().Debug("Stops writting: ", f.name)

	return uint32(written), fuse.OK
}

func (f *storjFile) getReader(off int64) (io.ReadCloser, error) {
	if f.reader == nil {
		readOnlyStream, err := f.metainfo.GetObjectStream(f.ctx, f.bucket.Name, f.name)
		if err != nil {
			return nil, err
		}

		download := stream.NewDownload(f.ctx, readOnlyStream, f.streams)
		_, err = download.Seek(off, io.SeekStart)
		if err != nil {
			return nil, err
		}

		f.reader = download
	}
	return f.reader, nil
}

func (f *storjFile) getWriter(off int64) (io.Writer, error) {
	if off == 0 {
		f.size = 0
		f.closeWriter()

		createInfo := storj.CreateObject{
			RedundancyScheme: cfg.GetRedundancyScheme(),
			EncryptionScheme: cfg.GetEncryptionScheme(),
		}
		var err error
		f.mutableObject, err = f.metainfo.CreateObject(f.ctx, f.bucket.Name, f.name, &createInfo)
		if err != nil {
			return nil, err
		}

		mutableStream, err := f.mutableObject.CreateStream(f.ctx)
		if err != nil {
			return nil, err
		}

		f.writer = stream.NewUpload(f.ctx, mutableStream, f.streams)
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
		closeErr := f.reader.Close()
		if closeErr != nil {
			zap.S().Errorf("error closing reader: %v", closeErr)
		}
		f.reader = nil
	}
}

func (f *storjFile) closeWriter() {
	if f.writer != nil {
		closeErr := f.writer.Close()
		if closeErr != nil {
			zap.S().Errorf("error closing writer: %v", closeErr)
		}

		f.FS.removeCreatedFile(f.name)
		err := f.mutableObject.Commit(f.ctx)
		if err != nil {
			zap.S().Errorf("error during commiting data: %v", err)
		}
		f.writer = nil
	}
}
