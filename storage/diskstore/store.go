package diskstore

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"os"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

// Error is the default diskstore error class
var Error = errs.Class("diskstore error")

const (
	headerSize = 32

	// TODO: implement readBufferSize  = 64 << 10 // 64 KB
	writeBufferSize = 64 << 10 // 64 KB
)

// Store implements a blob store
type Store struct {
	Disk *Disk
}

// New creates a new disk blob store on the specified disk
func New(disk *Disk) *Store {
	return &Store{disk}
}

// Load loads blob with the specified hash
func (store *Store) Load(ctx context.Context, hash storage.BlobRef) (storage.ReadSeekCloser, error) {
	file, openErr := store.Disk.Open(hash)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return nil, openErr
		}
		return nil, Error.Wrap(openErr)
	}

	bodyReader, err := newBlobBodyReader(file)
	if err != nil {
		return nil, Error.Wrap(utils.CombineErrors(err, file.Close()))
	}

	return bodyReader, nil
}

// Delete deletes blobs with the specified hash
func (store *Store) Delete(ctx context.Context, hash storage.BlobRef) error {
	err := store.Disk.Delete(hash)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// GarbageCollect tries to delete any files that haven't yet been deleted
func (store *Store) GarbageCollect(ctx context.Context) error {
	err := store.Disk.GarbageCollect()
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// Store stores r to disk, optionally takes a size argument, -1 is unknown size
func (store *Store) Store(ctx context.Context, r io.Reader, size int64) (storage.BlobRef, error) {
	file, err := store.Disk.CreateTemporaryFile(size)
	if err != nil {
		return storage.BlobRef{}, Error.Wrap(err)
	}

	hasher := sha256.New()
	bufferedFile := bufio.NewWriterSize(file, writeBufferSize)

	// write to both hasher and file
	writer := io.MultiWriter(hasher, bufferedFile)

	// seed file with random data
	_, err = io.CopyN(writer, rand.Reader, headerSize)
	if err != nil {
		return storage.BlobRef{}, Error.Wrap(err)
	}

	// copy data to disk
	if size >= 0 {
		if _, err = io.CopyN(writer, r, size); err != nil && err != io.EOF {
			return storage.BlobRef{}, Error.Wrap(utils.CombineErrors(err, store.Disk.DeleteTemporary(file)))
		}
	} else {
		if _, err = io.Copy(writer, r); err != nil && err != io.EOF {
			return storage.BlobRef{}, Error.Wrap(utils.CombineErrors(err, store.Disk.DeleteTemporary(file)))
		}
	}

	// flush any pending data
	if err = bufferedFile.Flush(); err != nil {
		return storage.BlobRef{}, Error.Wrap(err)
	}

	// figure out the hash
	var blobref storage.BlobRef
	copy(blobref[:], hasher.Sum(nil))

	// commit file to blob folder
	err = store.Disk.Commit(file, blobref)
	if err != nil {
		return storage.BlobRef{}, Error.Wrap(err)
	}

	// return the reference
	return blobref, nil
}
