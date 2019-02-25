// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

// Error is the default filestore error class
var Error = errs.Class("filestore error")

const (
	headerSize = 32

	// TODO: implement readBufferSize  = 64 << 10 // 64 KB
	writeBufferSize = 64 << 10 // 64 KB
)

var _ storage.Blobs = (*Store)(nil)

// Store implements a blob store
type Store struct {
	dir *Dir
}

// New creates a new disk blob store in the specified directory
func New(dir *Dir) *Store {
	return &Store{dir}
}

// NewAt creates a new disk blob store in the specified directory
func NewAt(path string) (*Store, error) {
	dir, err := NewDir(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &Store{dir}, nil
}

// Load loads blob with the specified hash
func (store *Store) Load(ctx context.Context, hash storage.BlobRef) (storage.ReadSeekCloser, error) {
	file, openErr := store.dir.Open(hash)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return nil, openErr
		}
		return nil, Error.Wrap(openErr)
	}

	bodyReader, err := newBlobBodyReader(file)
	if err != nil {
		return nil, Error.Wrap(errs.Combine(err, file.Close()))
	}

	return bodyReader, nil
}

// Delete deletes blobs with the specified hash
func (store *Store) Delete(ctx context.Context, hash storage.BlobRef) error {
	err := store.dir.Delete(hash)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// GarbageCollect tries to delete any files that haven't yet been deleted
func (store *Store) GarbageCollect(ctx context.Context) error {
	err := store.dir.GarbageCollect()
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// Store stores r to disk, optionally takes a size argument, -1 is unknown size
func (store *Store) Store(ctx context.Context, r io.Reader, size int64) (storage.BlobRef, error) {
	file, err := store.dir.CreateTemporaryFile(size)
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
			return storage.BlobRef{}, Error.Wrap(errs.Combine(err, store.dir.DeleteTemporary(file)))
		}
	} else {
		if _, err = io.Copy(writer, r); err != nil && err != io.EOF {
			return storage.BlobRef{}, Error.Wrap(errs.Combine(err, store.dir.DeleteTemporary(file)))
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
	err = store.dir.Commit(file, blobref)
	if err != nil {
		return storage.BlobRef{}, Error.Wrap(err)
	}

	// return the reference
	return blobref, nil
}
