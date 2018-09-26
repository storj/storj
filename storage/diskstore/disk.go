package diskstore

import (
	"encoding/hex"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

const (
	blobPermission = 0755
	dirPermission  = 0755
)

// Disk represents single folder for storing blobs
type Disk struct {
	dir string

	mu          sync.Mutex
	deleteQueue []string
}

// NewDisk returns folder for storing blobs
func NewDisk(dir string) (*Disk, error) {
	disk := &Disk{
		dir: dir,
	}

	return disk, utils.CombineErrors(
		os.MkdirAll(disk.blobdir(), dirPermission),
		os.MkdirAll(disk.tempdir(), dirPermission),
		os.MkdirAll(disk.trashdir(), dirPermission),
	)
}

func (disk *Disk) Dir() string      { return disk.dir }
func (disk *Disk) blobdir() string  { return filepath.Join(disk.dir) }
func (disk *Disk) tempdir() string  { return filepath.Join(disk.dir, "tmp") }
func (disk *Disk) trashdir() string { return filepath.Join(disk.dir, "trash") }

// CreateTemporaryFile creates a preallocated temporary file in the temp directory
func (disk *Disk) CreateTemporaryFile(prealloc int64) (*os.File, error) {
	file, err := ioutil.TempFile(disk.tempdir(), "blob-*.partial")
	if err != nil {
		return nil, err
	}

	if prealloc >= 0 {
		if err := file.Truncate(prealloc); err != nil {
			return nil, utils.CombineErrors(err, file.Close())
		}
	}
	return file, nil
}

// DeleteTemporary deletes a temporary file
func (disk *Disk) DeleteTemporary(file *os.File) error {
	closeErr := file.Close()
	return utils.CombineErrors(closeErr, os.Remove(file.Name()))
}

// refToPath converts blob reference to a filepath
func (disk *Disk) refToPath(ref storage.BlobRef) string {
	hex := hex.EncodeToString(ref[:])
	return filepath.Join(disk.blobdir(), hex[0:2], hex[2:])
}

// Commit commits temporary file to the permanent storage
func (disk *Disk) Commit(file *os.File, ref storage.BlobRef) error {
	position, seekErr := file.Seek(0, os.SEEK_CUR)
	truncErr := file.Truncate(position)
	syncErr := file.Sync()
	var chmodErr error
	if runtime.GOOS != "windows" {
		chmodErr = file.Chmod(blobPermission)
	}
	closeErr := file.Close()

	if seekErr != nil || truncErr != nil || syncErr != nil || chmodErr != nil || closeErr != nil {
		removeErr := os.Remove(file.Name())
		return utils.CombineErrors(seekErr, truncErr, syncErr, chmodErr, closeErr, removeErr)
	}

	path := disk.refToPath(ref)
	mkdirErr := os.MkdirAll(filepath.Dir(path), dirPermission)
	if os.IsExist(mkdirErr) {
		mkdirErr = nil
	}
	if mkdirErr != nil {
		removeErr := os.Remove(file.Name())
		return utils.CombineErrors(mkdirErr, removeErr)
	}

	renameErr := os.Rename(file.Name(), path)
	if renameErr != nil {
		removeErr := os.Remove(file.Name())
		return utils.CombineErrors(renameErr, removeErr)
	}

	return nil
}

// Open opens the file with the specified ref
func (disk *Disk) Open(ref storage.BlobRef) (*os.File, error) {
	path := disk.refToPath(ref)
	return os.OpenFile(path, os.O_RDONLY, blobPermission)
}

// Delete deletes file with the specified ref
func (disk *Disk) Delete(ref storage.BlobRef) error {
	path := disk.refToPath(ref)

	// move to trash folder, this is allowed for some OS-es
	trashPath := filepath.Join(disk.trashdir(), hex.EncodeToString(ref[:]))
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
		disk.mu.Lock()
		disk.deleteQueue = append(disk.deleteQueue, trashPath)
		disk.mu.Unlock()
	}

	// ignore is busy errors, they are still in the queue
	// but no need to notify
	if isBusy(err) {
		err = nil
	}

	return err
}

// GarbageCollect collects files that are pending deletion
func (disk *Disk) GarbageCollect() error {
	offset := int(math.MaxInt32)
	// limited deletion loop to avoid blocking `Delete` for too long
	for offset >= 0 {
		disk.mu.Lock()
		limit := 100
		if offset >= len(disk.deleteQueue) {
			offset = len(disk.deleteQueue) - 1
		}
		for i := offset; i >= 0 && limit > 0; i-- {
			limit--

			path := disk.deleteQueue[i]
			err := os.Remove(path)
			if os.IsNotExist(err) {
				err = nil
			}
			if err == nil {
				disk.deleteQueue = append(disk.deleteQueue[:i], disk.deleteQueue[i+1:]...)
			}
		}
		disk.mu.Unlock()
	}

	// remove anything left in the trashdir
	_ = removeAllContent(disk.trashdir())
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
	}
}
