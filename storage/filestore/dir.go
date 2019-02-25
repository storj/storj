// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"encoding/hex"
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
		os.MkdirAll(dir.blobdir(), dirPermission),
		os.MkdirAll(dir.tempdir(), dirPermission),
		os.MkdirAll(dir.trashdir(), dirPermission),
	)
}

// Path returns the directory path
func (dir *Dir) Path() string { return dir.path }

func (dir *Dir) blobdir() string  { return filepath.Join(dir.path) }
func (dir *Dir) tempdir() string  { return filepath.Join(dir.path, "tmp") }
func (dir *Dir) trashdir() string { return filepath.Join(dir.path, "trash") }

// CreateTemporaryFile creates a preallocated temporary file in the temp directory
// prealloc preallocates file to make writing faster
func (dir *Dir) CreateTemporaryFile(prealloc int64) (*os.File, error) {
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
func (dir *Dir) DeleteTemporary(file *os.File) error {
	closeErr := file.Close()
	return errs.Combine(closeErr, os.Remove(file.Name()))
}

// refToPath converts blob reference to a filepath
func (dir *Dir) refToPath(ref storage.BlobRef) string {
	hex := hex.EncodeToString(ref[:])
	return filepath.Join(dir.blobdir(), hex[0:2], hex[2:])
}

// Commit commits temporary file to the permanent storage
func (dir *Dir) Commit(file *os.File, ref storage.BlobRef) error {
	position, seekErr := file.Seek(0, io.SeekCurrent)
	truncErr := file.Truncate(position)
	syncErr := file.Sync()
	chmodErr := os.Chmod(file.Name(), blobPermission)
	closeErr := file.Close()

	if seekErr != nil || truncErr != nil || syncErr != nil || chmodErr != nil || closeErr != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(seekErr, truncErr, syncErr, chmodErr, closeErr, removeErr)
	}

	path := dir.refToPath(ref)
	mkdirErr := os.MkdirAll(filepath.Dir(path), dirPermission)
	if os.IsExist(mkdirErr) {
		mkdirErr = nil
	}
	if mkdirErr != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(mkdirErr, removeErr)
	}

	renameErr := os.Rename(file.Name(), path)
	if renameErr != nil {
		removeErr := os.Remove(file.Name())
		return errs.Combine(renameErr, removeErr)
	}

	return nil
}

// Open opens the file with the specified ref
func (dir *Dir) Open(ref storage.BlobRef) (*os.File, error) {
	path := dir.refToPath(ref)
	return os.OpenFile(path, os.O_RDONLY, blobPermission)
}

// Delete deletes file with the specified ref
func (dir *Dir) Delete(ref storage.BlobRef) error {
	path := dir.refToPath(ref)

	// move to trash folder, this is allowed for some OS-es
	trashPath := filepath.Join(dir.trashdir(), hex.EncodeToString(ref[:]))
	moveErr := os.Rename(path, trashPath)

	// ignore concurrent delete
	if os.IsNotExist(moveErr) {
		return nil
	}
	if moveErr != nil {
		trashPath = path
	}

	// try removing the file
	err := os.Remove(trashPath)

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
func (dir *Dir) GarbageCollect() error {
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
	_ = removeAllContent(dir.trashdir())
	return nil
}

// removeAllContent deletes everything in the folder
func removeAllContent(path string) error {
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
	return diskInfoFromPath(dir.path)
}
