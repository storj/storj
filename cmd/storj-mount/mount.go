// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// usage: storj-sim sj://bucket_name ~/target-directory
//
// Currently storj-mount is using 'uplink' configuration

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/version"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
	"storj.io/storj/uplink/setup"
)

const (
	badHandle         = ^uint64(0)
	normalMode uint32 = 00777
	fuseOK            = 0
)

// UplinkFlags test
type UplinkFlags struct {
	uplink.Config

	Version version.Config
}

var (
	cfg UplinkFlags

	rootCmd = &cobra.Command{
		Use:   "mount",
		Short: "Storj mount utility",
		Args:  cobra.OnlyValidArgs,
	}

	mountCmd = &cobra.Command{
		Use:   "run",
		Short: "to run mount utility",
		RunE:  mountBucket,
		Args:  cobra.OnlyValidArgs,
	}
)

func main() {
	process.Exec(rootCmd)
}

func init() {
	rootCmd.AddCommand(mountCmd)

	defaultConfDir := fpath.ApplicationDir("storj", "uplink")

	var confDir string
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for uplink configuration")
	defaults := cfgstruct.DefaultsFlag(rootCmd)

	process.Bind(mountCmd, &cfg, defaults, cfgstruct.ConfDir(confDir))
}

func mountBucket(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("No bucket specified for mounting")
	}
	if len(args) == 1 {
		return fmt.Errorf("No destination specified")
	}

	ctx := process.Ctx(cmd)

	if err := process.InitMetrics(ctx, nil, ""); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher: ", err)
	}

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}
	if src.IsLocal() {
		return fmt.Errorf("No bucket specified. Use format sj://bucket/")
	}

	access, err := setup.LoadEncryptionAccess(ctx, cfg.Enc)
	if err != nil {
		return err
	}

	project, bucket, err := cfg.GetProjectAndBucket(ctx, src.Bucket(), access)
	if err != nil {
		return fmt.Errorf("Unable to get bucket: %v", err)
	}
	defer closeProjectAndBucket(project, bucket)

	memfs := newStorjFS(ctx, bucket)
	host := fuse.NewFileSystemHost(memfs)
	host.SetCapReaddirPlus(true)
	if host.Mount("h:", []string{}) {
		<-ctx.Done()
		if !host.Unmount() {
			fmt.Printf("Unmount failed")
		}
	}

	return nil
}

type storjFS struct {
	ctx          context.Context
	bucket       *libuplink.Bucket
	createdFiles map[uint64]*storjFile
	fuse.FileSystemBase
}

func newStorjFS(ctx context.Context, bucket *libuplink.Bucket) *storjFS {
	return &storjFS{
		ctx:          ctx,
		bucket:       bucket,
		createdFiles: make(map[uint64]*storjFile),
	}
}

//Statfs gets file system stats
func (sf *storjFS) Statfs(path string, stat *fuse.Statfs_t) (errc int) {
	zap.S().Debug("Statfs: ", path)

	*stat = fuse.Statfs_t{
		//see http://man7.org/linux/man-pages/man2/statfs.2.html
		Bsize:   1,
		Frsize:  1,
		Blocks:  500000000,
		Bfree:   400000000,
		Bavail:  400000000,
		Files:   100000,
		Ffree:   100000,
		Favail:  100000,
		Fsid:    0,
		Flag:    0,
		Namemax: 256,
	}
	return fuseOK
}

// Getattr gets file attributes.
func (sf *storjFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	//zap.S().Debug("GetAttr: ", path)

	if path == "/" {
		*stat = *sf.newStat(true, 0, 1)
		return fuseOK
	}

	// special case for just created files e.g. while coping into directory
	// _, ok := sf.createdFiles[path]
	// if ok {
	// 	//*stat = createdFile.GetAttr(stat)
	// 	*stat = *sf.newStat(false, 0, fh)
	// 	return fuseOK
	// }

	// node := sf.nodeFS.Node(path)
	// if node != nil && node.IsDir() {
	// 	stat = sf.newStat(true, 0, fh)
	// 	return fuseOK
	// }

	object, err := sf.bucket.OpenObject(sf.ctx, path[1:])
	if err != nil && !storj.ErrObjectNotFound.Has(err) {
		stat = nil
		return fuse.EIO
	}

	// file not found so maybe it's a prefix/directory
	if err != nil {
		list, err2 := sf.bucket.ListObjects(sf.ctx, &storj.ListOptions{
			Prefix:    path[1:], //remove slash,
			Direction: storj.After,
			Limit:     1,
		})
		if err2 != nil {
			stat = nil
			return fuse.EIO
		}

		// if there's stuff beneath it, its a directory
		if len(list.Items) == 1 {
			*stat = *sf.newStat(true, 0, fh)
			return fuseOK
		}
		stat = nil
		return fuse.ENOENT
	}

	*stat = *sf.newStat(false, object.Meta.Size, fh)
	//Mtime: uint64(object.Meta.Modified.Unix())
	return fuseOK
}

// Readdir reads a directory.
func (sf *storjFS) Readdir(path string, fill func(path string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	zap.S().Debug("Readdir: ", path)

	fill(".", sf.newStat(true, 0, fh), 0)
	fill("..", nil, 0)
	err := sf.listObjects(sf.ctx, path, false, func(items []storj.Object) error {
		for _, item := range items {
			stat := sf.newStat(item.IsPrefix, 0, badHandle)
			if item.IsPrefix {
				item.Path = item.Path[:len(item.Path)-1] //remove end slash
			} else {
				stat.Size = item.Size
			}
			if !fill(item.Path, stat, 0) {
				//todo:  combine?
				return fmt.Errorf("error during fill directory: %s", item.Path)
			}
		}
		return nil
	})
	if err != nil {
		zap.S().Errorf("error during opening directory: %v", err)
		return fuse.EIO
	}

	return fuseOK
}

// newStat returns a fuse.Stat_t construct
func (sf *storjFS) newStat(isDir bool, size int64, serial uint64) *fuse.Stat_t {
	uid, gid, _ := fuse.Getcontext()
	var mode = normalMode
	if isDir {
		mode |= fuse.S_IFDIR
	}
	if serial == 1 {
		uid, gid = 0, 0
	}
	tmsp := fuse.Now()
	return &fuse.Stat_t{
		Dev:      0,
		Ino:      serial,
		Mode:     mode,
		Nlink:    1,
		Uid:      uid,
		Gid:      gid,
		Atim:     tmsp,
		Mtim:     tmsp,
		Ctim:     tmsp,
		Birthtim: tmsp,
		Flags:    0,
		Size:     size,
	}
}

// Mkdir creates a directory.
func (sf *storjFS) Mkdir(path string, mode uint32) int {
	zap.S().Debug("Mkdir: ", path)

	emptyReader := bytes.NewReader([]byte{})
	err := sf.bucket.UploadObject(sf.ctx, path+"/", emptyReader, &libuplink.UploadOptions{
		ContentType: "application/directory",
	})
	if err != nil {
		return fuse.EIO
	}

	return fuseOK
}

// Rmdir removes a directory.
func (sf *storjFS) Rmdir(path string) int {
	zap.S().Debug("Rmdir: ", path)

	err := sf.listObjects(sf.ctx, path, true, func(items []storj.Object) error {
		for _, item := range items {
			err := sf.bucket.DeleteObject(sf.ctx, storj.JoinPaths(path, item.Path))
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

	return fuseOK
}

// Open opens a file.
// The flags are a combination of the fuse.O_* constants.
func (sf *storjFS) Open(path string, flags int) (int, uint64) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fh := r.Uint64()
	zap.S().Debug("Open: ", path, " : ", fh)

	object, err := sf.bucket.OpenObject(sf.ctx, path[1:])
	if err != nil && !storj.ErrObjectNotFound.Has(err) {
		return fuse.EIO, badHandle
	}

	file := newStorjFile(sf.ctx, path, sf.bucket, false, sf)
	file.size = uint64(object.Meta.Size)

	sf.createdFiles[fh] = file
	return fuseOK, fh
}

// Create creates and opens a file.
// The flags are a combination of the fuse.O_* constants.
func (sf *storjFS) Create(path string, flags int, mode uint32) (int, uint64) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fh := r.Uint64()
	zap.S().Debug("Create: ", path, " : ", fh)

	file := newStorjFile(sf.ctx, path, sf.bucket, true, sf)
	sf.createdFiles[fh] = file
	return fuseOK, badHandle
}

func (sf *storjFS) listObjects(ctx context.Context, path string, recursive bool, handler func([]storj.Object) error) error {
	startAfter := ""

	for {
		list, err := sf.bucket.ListObjects(sf.ctx, &storj.ListOptions{
			Direction: storj.After,
			Cursor:    startAfter,
			Prefix:    path[1:], //remove slash
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

// Unlink removes a file.
func (sf *storjFS) Unlink(path string) int {
	zap.S().Debug("Unlink: ", path)

	err := sf.bucket.DeleteObject(sf.ctx, path)
	if err != nil {
		if storj.ErrObjectNotFound.Has(err) {
			return fuse.ENOENT
		}
		return fuse.EIO
	}

	return fuseOK
}

type storjFile struct {
	ctx             context.Context
	bucket          *libuplink.Bucket
	created         bool
	path            string
	size            uint64
	mtime           uint64
	reader          io.ReadCloser
	writer          io.WriteCloser
	predictedOffset int64
	fs              *storjFS
	object          *libuplink.Object
}

func newStorjFile(ctx context.Context, path string, bucket *libuplink.Bucket, created bool, fs *storjFS) *storjFile {
	object, err := bucket.OpenObject(ctx, path[1:])
	if err != nil {
		return nil
	}
	return &storjFile{
		path:    path,
		ctx:     ctx,
		bucket:  bucket,
		mtime:   uint64(time.Now().Unix()),
		created: created,
		fs:      fs,
		object:  object,
	}
}

func (sf *storjFS) getFile(fh uint64) *storjFile {
	file, ok := sf.createdFiles[fh]
	if ok {
		return file
	}
	return nil
}

// Getattr gets file attributes.
// func (sf *storjFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
// 	zap.S().Debug("GetAttr file: ", path)

// 	if f.created {
// 		attr.Mode = fuse.S_IFREG | 0644
// 		if f.size != 0 {
// 			attr.Size = f.size
// 		}
// 		attr.Mtime = f.mtime
// 		return fuseOK
// 	}
// 	return fuse.ENOSYS
// }

// Read reads data from a file.
func (sf *storjFS) Read(path string, buff []byte, off int64, fh uint64) int {
	f := sf.getFile(fh)

	zap.S().Debug("Read: ", path, " : ", fh, " -- ", off, "/", len(buff))

	if f == nil {
		return 0
	}

	// Detect if offset was moved manually (e.g. stream rev/fwd)
	// if off != f.predictedOffset {
	// 	f.closeReader(fh)
	// }

	//reader, err := f.getReader(off)
	toGet := uint64(len(buff))
	if toGet+uint64(off) > f.size {
		toGet = f.size - uint64(off)
	}
	reader, err := f.object.DownloadRange(f.ctx, off, int64(toGet))
	if err != nil {
		return 0
	}

	if err != nil {
		if storj.ErrObjectNotFound.Has(err) {
			return 0
		}
		return 0
	}

	n, err := io.ReadAtLeast(reader, buff, int(toGet))
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0
	}
	if uint64(n) != toGet {
		return len(buff)
	}
	f.predictedOffset = off + int64(n)
	return n
}

// Write writes data to a file.
func (sf *storjFS) Write(path string, buff []byte, off int64, fh uint64) int {
	zap.S().Debug("Write: ", path)

	f := sf.getFile(fh)
	writer, err := f.getWriter(off, fh)
	if err != nil {
		return 0
	}

	zap.S().Debug("Starts writting: ", f.path)

	written, err := writer.Write(buff)
	if err != nil {
		return 0
	}

	f.size += uint64(written)

	zap.S().Debug("Stops writting: ", f.path)

	return written
}

func (f *storjFile) getReader(off int64) (io.ReadCloser, error) {
	if f.reader == nil {
		object, err := f.bucket.OpenObject(f.ctx, f.path[1:])
		if err != nil {
			return nil, err
		}
		reader, err := object.DownloadRange(f.ctx, off, object.Meta.Size-off)
		if err != nil {
			return nil, err
		}

		f.reader = reader
	}
	return f.reader, nil
}

func (f *storjFile) getWriter(off int64, fh uint64) (_ io.Writer, err error) {
	if off == 0 {
		f.size = 0
		f.closeWriter(fh)

		f.writer, err = f.bucket.NewWriter(f.ctx, f.path, &libuplink.UploadOptions{})
	}
	return f.writer, err
}

// Flush flushes cached file data.
func (sf *storjFS) Flush(path string, fh uint64) int {
	zap.S().Debug("Flush: ", path, " : ", fh)
	f := sf.getFile(fh)
	f.closeReader(fh)
	f.closeWriter(fh)
	return fuseOK
}

// Release closes an open file.
func (sf *storjFS) Release(path string, fh uint64) int {
	zap.S().Debug("Release: ", path, " : ", fh)
	f := sf.getFile(fh)
	//if f != nil {
	f.closeReader(fh)
	f.closeWriter(fh)
	delete(sf.createdFiles, fh)
	//}
	return fuseOK
}

func (f *storjFile) closeReader(fh uint64) {
	if f.reader != nil {
		closeErr := f.reader.Close()
		if closeErr != nil {
			zap.S().Errorf("error closing reader: %v", closeErr)
		}
		f.reader = nil
	}
}

func (f *storjFile) closeWriter(fh uint64) {
	if f.writer != nil {
		closeErr := f.writer.Close()
		if closeErr != nil {
			zap.S().Errorf("error closing writer: %v", closeErr)
		}

		//delete(f.fs.createdFiles, fh)
		f.writer = nil
	}
}

// NewUplink returns a pointer to a new Client with a Config and Uplink pointer on it and an error.
func (cliCfg *UplinkFlags) NewUplink(ctx context.Context) (*libuplink.Uplink, error) {

	// Transform the uplink cli config flags to the libuplink config object
	libuplinkCfg := &libuplink.Config{}
	libuplinkCfg.Volatile.MaxInlineSize = cliCfg.Client.MaxInlineSize
	libuplinkCfg.Volatile.MaxMemory = cliCfg.RS.MaxBufferMem
	libuplinkCfg.Volatile.PeerIDVersion = cliCfg.TLS.PeerIDVersions
	libuplinkCfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: !cliCfg.TLS.UsePeerCAWhitelist,
		PeerCAWhitelistPath: cliCfg.TLS.PeerCAWhitelistPath,
	}

	libuplinkCfg.Volatile.DialTimeout = cliCfg.Client.DialTimeout
	libuplinkCfg.Volatile.RequestTimeout = cliCfg.Client.RequestTimeout

	return libuplink.NewUplink(ctx, libuplinkCfg)
}

// GetProject returns a *libuplink.Project for interacting with a specific project
func (cliCfg *UplinkFlags) GetProject(ctx context.Context) (*libuplink.Project, error) {
	err := version.CheckProcessVersion(ctx, cliCfg.Version, version.Build, "Uplink")
	if err != nil {
		return nil, err
	}

	apiKey, err := libuplink.ParseAPIKey(cliCfg.Client.APIKey)
	if err != nil {
		return nil, err
	}

	uplk, err := cliCfg.NewUplink(ctx)
	if err != nil {
		return nil, err
	}

	project, err := uplk.OpenProject(ctx, cliCfg.Client.SatelliteAddr, apiKey)
	if err != nil {
		if err := uplk.Close(); err != nil {
			fmt.Printf("error closing uplink: %+v\n", err)
		}
	}

	return project, err
}

// GetProjectAndBucket returns a *libuplink.Bucket for interacting with a specific project's bucket
func (cliCfg *UplinkFlags) GetProjectAndBucket(ctx context.Context, bucketName string, access *libuplink.EncryptionAccess) (project *libuplink.Project, bucket *libuplink.Bucket, err error) {
	project, err = cliCfg.GetProject(ctx)
	if err != nil {
		return project, bucket, err
	}

	defer func() {
		if err != nil {
			if err := project.Close(); err != nil {
				fmt.Printf("error closing project: %+v\n", err)
			}
		}
	}()

	bucket, err = project.OpenBucket(ctx, bucketName, access)
	if err != nil {
		return project, bucket, err
	}

	return project, bucket, err
}

func closeProjectAndBucket(project *libuplink.Project, bucket *libuplink.Bucket) {
	if err := bucket.Close(); err != nil {
		fmt.Printf("error closing bucket: %+v\n", err)
	}

	if err := project.Close(); err != nil {
		fmt.Printf("error closing project: %+v\n", err)
	}
}

// Init is called when the file system is created.
func (sf *storjFS) Init() {
	sf.FileSystemBase.Init()
}

// Destroy is called when the file system is destroyed.
func (sf *storjFS) Destroy() {
	sf.FileSystemBase.Destroy()
}

// // Statfs gets file system statistics.
// func (sf *storjFS) Statfs(path string, stat *fuse.Statfs_t) int {
// 	return sf.FileSystemBase.Statfs(path, stat)
// }

// Mknod creates a file node.
func (sf *storjFS) Mknod(path string, mode uint32, dev uint64) int {
	return sf.FileSystemBase.Mknod(path, mode, dev)
}

// // Mkdir creates a directory.
// func (sf *storjFS) Mkdir(path string, mode uint32) int {
// 	return sf.FileSystemBase.Mkdir(path, mode)
// }

// // Unlink removes a file.
// func (sf *storjFS) Unlink(path string) int {
// 	return sf.FileSystemBase.Unlink(path)
// }

// // Rmdir removes a directory.
// func (sf *storjFS) Rmdir(path string) int {
// 	return sf.FileSystemBase.Rmdir(path)
// }

// Link creates a hard link to a file.
func (sf *storjFS) Link(oldpath string, newpath string) int {
	return sf.FileSystemBase.Link(oldpath, newpath)
}

// Symlink creates a symbolic link.
func (sf *storjFS) Symlink(target string, newpath string) int {
	return sf.FileSystemBase.Symlink(target, newpath)
}

// Readlink reads the target of a symbolic link.
func (sf *storjFS) Readlink(path string) (int, string) {
	return sf.FileSystemBase.Readlink(path)
}

// Rename renames a file.
func (sf *storjFS) Rename(oldpath string, newpath string) int {
	return sf.FileSystemBase.Rename(oldpath, newpath)
}

// Chmod changes the permission bits of a file.
func (sf *storjFS) Chmod(path string, mode uint32) int {
	return sf.FileSystemBase.Chmod(path, mode)
}

// Chown changes the owner and group of a file.
func (sf *storjFS) Chown(path string, uid uint32, gid uint32) int {
	return sf.FileSystemBase.Chown(path, uid, gid)
}

// Utimens changes the access and modification times of a file.
func (sf *storjFS) Utimens(path string, tmsp []fuse.Timespec) int {
	return sf.FileSystemBase.Utimens(path, tmsp)
}

// Access checks file access permissions.
func (sf *storjFS) Access(path string, mask uint32) int {
	return sf.FileSystemBase.Access(path, mask)
}

// // Create creates and opens a file.
// // The flags are a combination of the fuse.O_* constants.
// func (sf *storjFS) Create(path string, flags int, mode uint32) (int, uint64) {
// 	return sf.FileSystemBase.Create(path, flags, mode)
// }

// // Open opens a file.
// // The flags are a combination of the fuse.O_* constants.
// func (sf *storjFS) Open(path string, flags int) (int, uint64) {
// 	return sf.FileSystemBase.Open(path, flags)
// }

// // Getattr gets file attributes.
// func (sf *storjFS) Getattr(path string, stat *Stat_t, fh uint64) int {
// 	return sf.FileSystemBase.Getattr(path, stat)
// }

// Truncate changes the size of a file.
func (sf *storjFS) Truncate(path string, size int64, fh uint64) int {
	return sf.FileSystemBase.Truncate(path, size, fh)
}

// // Read reads data from a file.
// func (sf *storjFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
// 	return sf.FileSystemBase.Read(path, buff, ofst, fh)
// }

// // Write writes data to a file.
// func (sf *storjFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
// 	return sf.FileSystemBase.Write(path, buff, ofst, fh)
// }

// // Flush flushes cached file data.
// func (sf *storjFS) Flush(path string, fh uint64) int {
// 	return sf.FileSystemBase.Flush(path, fh)
// }

// // Release closes an open file.
// func (sf *storjFS) Release(path string, fh uint64) int {
// 	return sf.FileSystemBase.Releast(path, fh)
// }

// Fsync synchronizes file contents.
func (sf *storjFS) Fsync(path string, datasync bool, fh uint64) int {
	return sf.FileSystemBase.Fsync(path, datasync, fh)
}

// Lock performs a file locking operation.
//Lock(path string, cmd int, lock *Lock_t, fh uint64) int

// Opendir opens a directory.
func (sf *storjFS) Opendir(path string) (int, uint64) {
	return sf.FileSystemBase.Opendir(path)
}

// // Readdir reads a directory.
// func (sf *storjFS) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
// 	return sf.FileSystemBase.Readdir(path, fill, ofst, fh)
// }

// Releasedir closes an open directory.
func (sf *storjFS) Releasedir(path string, fh uint64) int {
	return sf.FileSystemBase.Releasedir(path, fh)
}

// Fsyncdir synchronizes directory contents.
func (sf *storjFS) Fsyncdir(path string, datasync bool, fh uint64) int {
	return sf.FileSystemBase.Fsyncdir(path, datasync, fh)
}

// Setxattr sets extended attributes.
func (sf *storjFS) Setxattr(path string, name string, value []byte, flags int) int {
	return sf.FileSystemBase.Setxattr(path, name, value, flags)
}

// Getxattr gets extended attributes.
func (sf *storjFS) Getxattr(path string, name string) (int, []byte) {
	return sf.FileSystemBase.Getxattr(path, name)
}

// Removexattr removes extended attributes.
func (sf *storjFS) Removexattr(path string, name string) int {
	return sf.FileSystemBase.Removexattr(path, name)
}

// Listxattr lists extended attributes.
func (sf *storjFS) Listxattr(path string, fill func(name string) bool) int {
	return sf.FileSystemBase.Listxattr(path, fill)
}
