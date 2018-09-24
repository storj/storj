package diskstore

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

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
}

// NewDisk returns folder for storing blobs
func NewDisk(dir string) (*Disk, error) {
	disk := &Disk{
		dir: dir,
	}

	blobErr := os.MkdirAll(disk.blobdir(), dirPermission)
	tempErr := os.MkdirAll(disk.tempdir(), dirPermission)

	return disk, utils.CombineErrors(blobErr, tempErr)
}

func (disk *Disk) Dir() string     { return disk.dir }
func (disk *Disk) blobdir() string { return filepath.Join(disk.dir, "blob") }
func (disk *Disk) tempdir() string { return filepath.Join(disk.dir, "tmp") }

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
	return os.Remove(path)
}
