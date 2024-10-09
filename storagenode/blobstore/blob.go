// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package blobstore

import (
	"context"
	"io"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ErrInvalidBlobRef is returned when an blob reference is invalid.
var ErrInvalidBlobRef = errs.Class("invalid blob ref")

// FormatVersion represents differing storage format version values. Different Blobs implementors
// might interpret different FormatVersion values differently, but they share a type so that there
// can be a common StorageFormatVersion() call on the interface.
//
// Changes in FormatVersion might affect how a Blobs or BlobReader or BlobWriter instance works, or
// they might only be relevant to some higher layer. A FormatVersion must be specified when writing
// a new blob, and the blob storage interface must store that value with the blob somehow, so that
// the same FormatVersion is returned later when reading that stored blob.
type FormatVersion int

// BlobRef is a reference to a blob.
type BlobRef struct {
	Namespace []byte
	Key       []byte
}

// IsValid returns whether both namespace and key are specified.
func (ref *BlobRef) IsValid() bool {
	return len(ref.Namespace) > 0 && len(ref.Key) > 0
}

// BlobReader is an interface that groups Read, ReadAt, Seek and Close.
type BlobReader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
	// Size returns the size of the blob.
	Size() (int64, error)
	// StorageFormatVersion returns the storage format version associated with the blob.
	StorageFormatVersion() FormatVersion
}

// BlobWriter defines the interface that must be satisfied for a general blob storage provider.
// BlobWriter instances are returned by the Create() method on Blobs instances.
type BlobWriter interface {
	io.Writer
	io.Seeker
	// Cancel discards the blob.
	Cancel(context.Context) error
	// Commit ensures that the blob is readable by others.
	Commit(context.Context) error
	// Size returns the size of the blob
	Size() (int64, error)
	// ReserveHeader reserves header area at the beginning of the blob.
	ReserveHeader(int64) error
	// StorageFormatVersion returns the storage format version associated with the blob.
	StorageFormatVersion() FormatVersion
}

// Blobs is a blob storage interface.
//
// architecture: Database
type Blobs interface {
	// Create creates a new blob that can be written.
	Create(ctx context.Context, ref BlobRef) (BlobWriter, error)
	// Open opens a reader with the specified namespace and key.
	Open(ctx context.Context, ref BlobRef) (BlobReader, error)
	// OpenWithStorageFormat opens a reader for the already-located blob, avoiding the potential
	// need to check multiple storage formats to find the blob.
	OpenWithStorageFormat(ctx context.Context, ref BlobRef, formatVer FormatVersion) (BlobReader, error)
	// Delete deletes the blob with the namespace and key.
	Delete(ctx context.Context, ref BlobRef) error
	// DeleteWithStorageFormat deletes a blob of a specific storage format.
	DeleteWithStorageFormat(ctx context.Context, ref BlobRef, formatVer FormatVersion, sizeHint int64) error
	// DeleteNamespace deletes blobs folder for a specific namespace.
	DeleteNamespace(ctx context.Context, ref []byte) (err error)
	// DeleteTrashNamespace deletes the trash folder for a given namespace.
	DeleteTrashNamespace(ctx context.Context, namespace []byte) (err error)
	// Trash marks a file for pending deletion.
	Trash(ctx context.Context, ref BlobRef, timestamp time.Time) error
	// TrashWithStorageFormat marks a blob with a specific storage format for pending deletion.
	TrashWithStorageFormat(ctx context.Context, ref BlobRef, formatVer FormatVersion, timestamp time.Time) error
	// RestoreTrash restores all files in the trash for a given namespace and returns the keys restored.
	RestoreTrash(ctx context.Context, namespace []byte) ([][]byte, error)
	// EmptyTrash removes all files in trash that were moved to trash prior to trashedBefore and returns the total bytes emptied and keys deleted.
	EmptyTrash(ctx context.Context, namespace []byte, trashedBefore time.Time) (int64, [][]byte, error)
	// TryRestoreTrashBlob attempts to restore a blob from the trash.
	// It returns nil if the blob was restored, or an error if the blob was not
	// in the trash or could not be restored.
	TryRestoreTrashBlob(ctx context.Context, ref BlobRef) error
	// Stat looks up disk metadata on the blob file.
	Stat(ctx context.Context, ref BlobRef) (BlobInfo, error)
	// StatWithStorageFormat looks up disk metadata for the blob file with the given storage format
	// version. This avoids the potential need to check multiple storage formats for the blob
	// when the format is already known.
	StatWithStorageFormat(ctx context.Context, ref BlobRef, formatVer FormatVersion) (BlobInfo, error)

	// DiskInfo returns information about the disk.
	DiskInfo(ctx context.Context) (DiskInfo, error)
	// SpaceUsedForTrash returns the total space used by the trash.
	SpaceUsedForTrash(ctx context.Context) (int64, error)
	// SpaceUsedForBlobs adds up how much is used in all namespaces.
	SpaceUsedForBlobs(ctx context.Context) (int64, error)
	// SpaceUsedForBlobsInNamespace adds up how much is used in the given namespace.
	SpaceUsedForBlobsInNamespace(ctx context.Context, namespace []byte) (int64, error)

	// ListNamespaces finds all namespaces in which keys might currently be stored.
	ListNamespaces(ctx context.Context) ([][]byte, error)
	// WalkNamespace executes walkFunc for each locally stored blob, stored with
	// storage format V1 or greater, in the given namespace. If walkFunc returns a non-nil
	// error, WalkNamespace will stop iterating and return the error immediately. The ctx
	// parameter is intended to allow canceling iteration early.
	WalkNamespace(ctx context.Context, namespace []byte, skipPrefixFn SkipPrefixFn, walkFunc func(BlobInfo) error) error

	// CheckWritability tests writability of the storage directory by creating and deleting a file.
	CheckWritability(ctx context.Context) error
	// CreateVerificationFile creates a file to be used for storage directory verification.
	CreateVerificationFile(ctx context.Context, id storj.NodeID) error
	// VerifyStorageDir verifies that the storage directory is correct by checking for the existence and validity
	// of the verification file.
	VerifyStorageDir(ctx context.Context, id storj.NodeID) error

	// Close closes the blob store and any resources associated with it.
	Close() error
}

// SkipPrefixFn returns true if a prefix should be skipped.
type SkipPrefixFn func(prefix string) bool

// BlobInfo allows lazy inspection of a blob and its underlying file during iteration with
// WalkNamespace-type methods.
type BlobInfo interface {
	// BlobRef returns the relevant BlobRef for the blob.
	BlobRef() BlobRef
	// StorageFormatVersion indicates the storage format version used to store the blob.
	StorageFormatVersion() FormatVersion
	// FullPath gives the full path to the on-disk blob file.
	FullPath(ctx context.Context) (string, error)
	// Stat does a stat on the on-disk blob file.
	Stat(ctx context.Context) (FileInfo, error)
}

// FileInfo contains information about the pieces files, what we care about.
type FileInfo interface {
	ModTime() time.Time
	Size() int64
}

// DiskInfo contains information about the disk.
type DiskInfo struct {
	TotalSpace     int64
	AvailableSpace int64
}
