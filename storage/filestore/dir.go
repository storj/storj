// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"context"
	"encoding/base32"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

const (
	blobPermission = 0600
	dirPermission  = 0700
)

var pathEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// Dir represents single folder for storing blobs
type Dir struct {
	path string

	mu          sync.Mutex
	deleteQueue []string
}

// NewDir returns folder for storing blobs
func NewDir(path string) (*Dir, error) {
	dir := &Dir{
		path: path,
	}

	return dir, errs.Combine(
		os.MkdirAll(dir.blobsdir(), dirPermission),
		os.MkdirAll(dir.tempdir(), dirPermission),
		os.MkdirAll(dir.garbagedir(), dirPermission),
	)
}

// Path returns the directory path
func (dir *Dir) Path() string { return dir.path }

func (dir *Dir) blobsdir() string   { return filepath.Join(dir.path, "blobs") }
func (dir *Dir) tempdir() string    { return filepath.Join(dir.path, "temp") }
func (dir *Dir) garbagedir() string { return filepath.Join(dir.path, "garbage") }

// CreateTemporaryFile creates a preallocated temporary file in the temp directory
// prealloc preallocates file to make writing faster
func (dir *Dir) CreateTemporaryFile(ctx context.Context, prealloc int64) (_ *os.File, err error) {
	const preallocLimit = 5 << 20 // 5 MB
	if prealloc > preallocLimit {
		prealloc = preallocLimit
	}

	file, err := ioutil.TempFile(dir.tempdir(), "blob-*.partial")
	if err != nil {
		return nil, err
	}

	if prealloc >= 0 {
		if err := file.Truncate(prealloc); err != nil {
			return nil, errs.Combine(err, file.Close())
		}
	}
	return file, nil
}

// DeleteTemporary deletes a temporary file
func (dir *Dir) DeleteTemporary(ctx context.Context, file *os.File) (err error) {
	defer mon.Task()(&ctx)(&err)
	closeErr := file.Close()
	return errs.Combine(closeErr, os.Remove(file.Name()))
}

// blobToPath converts blob reference to a filepath in permanent storage
func (dir *Dir) blobToPath(ref storage.BlobRef) (string, error) {
	if !ref.IsValid() {
		return "", storage.ErrInvalidBlobRef.New("")
	}

	namespace := pathEncoding.EncodeToString(ref.Namespace)
	key := pathEncoding.EncodeToString(ref.Key)
	if len(key) < 3 {
		// ensure we always have at least
		key = "11" + key
	}
	return filepath.Join(dir.blobsdir(), namespace, key[:2], key[2:]), nil
}

// blobToTrashPath converts blob reference to a filepath in transient storage
// the files in trash are deleted in an interval (in case the initial deletion didn't work for some reason)
func (dir *Dir) blobToTrashPath(ref storage.BlobRef) string {
	var name []byte
	name = append(name, ref.Namespace...)
	name = append(name, ref.Key...)
	return filepath.Join(dir.garbagedir(), pathEncoding.EncodeToString(name))
}

// Commit commits temporary file to the permanent storage
func (dir *Dir) Commit(ctx context.Context, file *os.File, ref storage.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	position, seekErr := file.Seek(0, io.SeekCurrent)
	truncErr := file.Truncate(position)
	syncErr := file.Sync()
	chmodErr := os.Chmod(file.Name(), blobPermission)
	closeErr := file.Close()

	if seekErr != nil || truncErr != nil || syncErr != nil || chmodErr != nil || closeErr != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(seekErr, truncErr, syncErr, chmodErr, closeErr, removeErr)
	}

	path, err := dir.blobToPath(ref)
	if err != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(err, removeErr)
	}

	mkdirErr := os.MkdirAll(filepath.Dir(path), dirPermission)
	if os.IsExist(mkdirErr) {
		mkdirErr = nil
	}

	if mkdirErr != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(mkdirErr, removeErr)
	}

	renameErr := rename(file.Name(), path)
	if renameErr != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(renameErr, removeErr)
	}

	return nil
}

// Open opens the file with the specified ref
func (dir *Dir) Open(ctx context.Context, ref storage.BlobRef) (_ *os.File, err error) {
	defer mon.Task()(&ctx)(&err)
	path, err := dir.blobToPath(ref)
	if err != nil {
		return nil, err
	}
	file, err := openFileReadOnly(path, blobPermission)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.New("unable to open %q: %v", path, err)
	}
	return file, nil
}

// Delete deletes file with the specified ref
func (dir *Dir) Delete(ctx context.Context, ref storage.BlobRef) (err error) {
	defer mon.Task()(&ctx)(&err)
	path, err := dir.blobToPath(ref)
	if err != nil {
		return err
	}

	trashPath := dir.blobToTrashPath(ref)

	// move to trash folder, this is allowed for some OS-es
	moveErr := rename(path, trashPath)

	// ignore concurrent delete
	if os.IsNotExist(moveErr) {
		return nil
	}
	if moveErr != nil {
		trashPath = path
	}

	// try removing the file
	err = os.Remove(trashPath)

	// ignore concurrent deletes
	if os.IsNotExist(err) {
		return nil
	}

	// this may fail, because someone might be still reading it
	if err != nil {
		dir.mu.Lock()
		dir.deleteQueue = append(dir.deleteQueue, trashPath)
		dir.mu.Unlock()
	}

	// ignore is busy errors, they are still in the queue
	// but no need to notify
	if isBusy(err) {
		err = nil
	}

	return err
}

// GarbageCollect collects files that are pending deletion
func (dir *Dir) GarbageCollect(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	offset := int(math.MaxInt32)
	// limited deletion loop to avoid blocking `Delete` for too long
	for offset >= 0 {
		dir.mu.Lock()
		limit := 100
		if offset >= len(dir.deleteQueue) {
			offset = len(dir.deleteQueue) - 1
		}
		for offset >= 0 && limit > 0 {
			path := dir.deleteQueue[offset]
			err := os.Remove(path)
			if os.IsNotExist(err) {
				err = nil
			}
			if err == nil {
				dir.deleteQueue = append(dir.deleteQueue[:offset], dir.deleteQueue[offset+1:]...)
			}

			offset--
			limit--
		}
		dir.mu.Unlock()
	}

	// remove anything left in the trashdir
	_ = removeAllContent(ctx, dir.garbagedir())
	return nil
}

// removeAllContent deletes everything in the folder
func removeAllContent(ctx context.Context, path string) (err error) {
	defer mon.Task()(&ctx)(&err)
	dir, err := os.Open(path)
	if err != nil {
		return err
	}

	for {
		files, err := dir.Readdirnames(100)
		for _, file := range files {
			// the file might be still in use, so ignore the error
			_ = os.RemoveAll(filepath.Join(path, file))
		}
		if err == io.EOF || len(files) == 0 {
			return dir.Close()
		}
		if err != nil {
			return err
		}
	}
}

// DiskInfo contains statistics about this dir
type DiskInfo struct {
	ID             string
	AvailableSpace int64
}

// Info returns information about the current state of the dir
func (dir *Dir) Info() (DiskInfo, error) {
	path, err := filepath.Abs(dir.path)
	if err != nil {
		return DiskInfo{}, err
	}
	return diskInfoFromPath(path)
}
