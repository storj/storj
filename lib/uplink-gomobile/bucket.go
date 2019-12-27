// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"fmt"
	"io"
	"time"

	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
)

const (
	// CipherSuiteEncUnspecified indicates no encryption suite has been selected.
	CipherSuiteEncUnspecified = byte(storj.EncUnspecified)
	// CipherSuiteEncNull indicates use of the NULL cipher; that is, no encryption is
	// done. The ciphertext is equal to the plaintext.
	CipherSuiteEncNull = byte(storj.EncNull)
	// CipherSuiteEncAESGCM indicates use of AES128-GCM encryption.
	CipherSuiteEncAESGCM = byte(storj.EncAESGCM)
	// CipherSuiteEncSecretBox indicates use of XSalsa20-Poly1305 encryption, as provided
	// by the NaCl cryptography library under the name "Secretbox".
	CipherSuiteEncSecretBox = byte(storj.EncSecretBox)

	// DirectionForward lists forwards from cursor, including cursor
	DirectionForward = int(storj.Forward)

	// DirectionAfter lists forwards from cursor, without cursor
	DirectionAfter = int(storj.After)
)

// Bucket represents operations you can perform on a bucket
type Bucket struct {
	Name string

	scope
	lib *libuplink.Bucket
}

// BucketInfo bucket meta struct
type BucketInfo struct {
	Name                 string
	Created              int64
	PathCipher           byte
	SegmentsSize         int64
	RedundancyScheme     *RedundancyScheme
	EncryptionParameters *EncryptionParameters
}

func newBucketInfo(bucket storj.Bucket) *BucketInfo {
	return &BucketInfo{
		Name:         bucket.Name,
		Created:      bucket.Created.UTC().UnixNano() / int64(time.Millisecond),
		PathCipher:   byte(bucket.PathCipher),
		SegmentsSize: bucket.DefaultSegmentsSize,
		RedundancyScheme: &RedundancyScheme{
			Algorithm:      byte(bucket.DefaultRedundancyScheme.Algorithm),
			ShareSize:      bucket.DefaultRedundancyScheme.ShareSize,
			RequiredShares: bucket.DefaultRedundancyScheme.RequiredShares,
			RepairShares:   bucket.DefaultRedundancyScheme.RepairShares,
			OptimalShares:  bucket.DefaultRedundancyScheme.OptimalShares,
			TotalShares:    bucket.DefaultRedundancyScheme.TotalShares,
		},
		EncryptionParameters: &EncryptionParameters{
			CipherSuite: byte(bucket.DefaultEncryptionParameters.CipherSuite),
			BlockSize:   bucket.DefaultEncryptionParameters.BlockSize,
		},
	}
}

// BucketConfig bucket configuration
type BucketConfig struct {
	// PathCipher indicates which cipher suite is to be used for path
	// encryption within the new Bucket. If not set, AES-GCM encryption
	// will be used.
	PathCipher byte

	// EncryptionParameters specifies the default encryption parameters to
	// be used for data encryption of new Objects in this bucket.
	EncryptionParameters *EncryptionParameters

	// RedundancyScheme defines the default Reed-Solomon and/or
	// Forward Error Correction encoding parameters to be used by
	// objects in this Bucket.
	RedundancyScheme *RedundancyScheme
	// SegmentsSize is the default segment size to use for new
	// objects in this Bucket.
	SegmentsSize int64
}

// BucketList is a list of buckets
type BucketList struct {
	list storj.BucketList
}

// More returns true if list request was not able to return all results
func (bl *BucketList) More() bool {
	return bl.list.More
}

// Length returns number of returned items
func (bl *BucketList) Length() int {
	return len(bl.list.Items)
}

// Item gets item from specific index
func (bl *BucketList) Item(index int) (*BucketInfo, error) {
	if index < 0 && index >= len(bl.list.Items) {
		return nil, fmt.Errorf("index out of range")
	}
	return newBucketInfo(bl.list.Items[index]), nil
}

// RedundancyScheme specifies the parameters and the algorithm for redundancy
type RedundancyScheme struct {
	// Algorithm determines the algorithm to be used for redundancy.
	Algorithm byte

	// ShareSize is the size to use for new redundancy shares.
	ShareSize int32

	// RequiredShares is the minimum number of shares required to recover a
	// segment.
	RequiredShares int16
	// RepairShares is the minimum number of safe shares that can remain
	// before a repair is triggered.
	RepairShares int16
	// OptimalShares is the desired total number of shares for a segment.
	OptimalShares int16
	// TotalShares is the number of shares to encode. If it is larger than
	// OptimalShares, slower uploads of the excess shares will be aborted in
	// order to improve performance.
	TotalShares int16
}

func newStorjRedundancyScheme(scheme *RedundancyScheme) storj.RedundancyScheme {
	if scheme == nil {
		return storj.RedundancyScheme{}
	}
	return storj.RedundancyScheme{
		Algorithm:      storj.RedundancyAlgorithm(scheme.Algorithm),
		ShareSize:      scheme.ShareSize,
		RequiredShares: scheme.RequiredShares,
		RepairShares:   scheme.RepairShares,
		OptimalShares:  scheme.OptimalShares,
		TotalShares:    scheme.TotalShares,
	}
}

// EncryptionParameters is the cipher suite and parameters used for encryption
type EncryptionParameters struct {
	// CipherSuite specifies the cipher suite to be used for encryption.
	CipherSuite byte
	// BlockSize determines the unit size at which encryption is performed.
	// It is important to distinguish this from the block size used by the
	// cipher suite (probably 128 bits). There is some small overhead for
	// each encryption unit, so BlockSize should not be too small, but
	// smaller sizes yield shorter first-byte latency and better seek times.
	// Note that BlockSize itself is the size of data blocks _after_ they
	// have been encrypted and the authentication overhead has been added.
	// It is _not_ the size of the data blocks to _be_ encrypted.
	BlockSize int32
}

func newStorjEncryptionParameters(ec *EncryptionParameters) storj.EncryptionParameters {
	if ec == nil {
		return storj.EncryptionParameters{}
	}
	return storj.EncryptionParameters{
		CipherSuite: storj.CipherSuite(ec.CipherSuite),
		BlockSize:   ec.BlockSize,
	}
}

// ListOptions options for listing objects
type ListOptions struct {
	Prefix    string
	Cursor    string // Cursor is relative to Prefix, full path is Prefix + Cursor
	Delimiter int32
	Recursive bool
	Direction int
	Limit     int
}

// ListObjects list objects in bucket, if authorized.
func (bucket *Bucket) ListObjects(options *ListOptions) (*ObjectList, error) {
	scope := bucket.scope.child()

	opts := &storj.ListOptions{}
	if options != nil {
		opts.Prefix = options.Prefix
		opts.Cursor = options.Cursor
		opts.Direction = storj.ListDirection(options.Direction)
		opts.Delimiter = options.Delimiter
		opts.Recursive = options.Recursive
		opts.Limit = options.Limit
	}

	list, err := bucket.lib.ListObjects(scope.ctx, opts)
	if err != nil {
		return nil, safeError(err)
	}
	return &ObjectList{list}, nil
}

// OpenObject returns an Object handle, if authorized.
func (bucket *Bucket) OpenObject(objectPath string) (*ObjectInfo, error) {
	scope := bucket.scope.child()
	object, err := bucket.lib.OpenObject(scope.ctx, objectPath)
	if err != nil {
		return nil, safeError(err)
	}
	return newObjectInfoFromObjectMeta(object.Meta), nil
}

// DeleteObject removes an object, if authorized.
func (bucket *Bucket) DeleteObject(objectPath string) error {
	scope := bucket.scope.child()
	return safeError(bucket.lib.DeleteObject(scope.ctx, objectPath))
}

// Close closes the Bucket session.
func (bucket *Bucket) Close() error {
	defer bucket.cancel()
	return safeError(bucket.lib.Close())
}

// WriterOptions controls options about writing a new Object
type WriterOptions struct {
	// ContentType, if set, gives a MIME content-type for the Object.
	ContentType string
	// Metadata contains additional information about an Object. It can
	// hold arbitrary textual fields and can be retrieved together with the
	// Object. Field names can be at most 1024 bytes long. Field values are
	// not individually limited in size, but the total of all metadata
	// (fields and values) can not exceed 4 kiB.
	Metadata map[string]string
	// Expires is the time at which the new Object can expire (be deleted
	// automatically from storage nodes).
	Expires int

	// EncryptionParameters determines the cipher suite to use for
	// the Object's data encryption. If not set, the Bucket's
	// defaults will be used.
	EncryptionParameters *EncryptionParameters

	// RedundancyScheme determines the Reed-Solomon and/or Forward
	// Error Correction encoding parameters to be used for this
	// Object.
	RedundancyScheme *RedundancyScheme
}

// NewWriterOptions creates writer options
func NewWriterOptions() *WriterOptions {
	return &WriterOptions{}
}

// Writer writes data into object
type Writer struct {
	scope
	writer io.WriteCloser
}

// NewWriter creates instance of Writer
func (bucket *Bucket) NewWriter(path storj.Path, options *WriterOptions) (*Writer, error) {
	scope := bucket.scope.child()

	opts := &libuplink.UploadOptions{}
	if options != nil {
		opts.ContentType = options.ContentType
		opts.Metadata = options.Metadata
		if options.Expires != 0 {
			opts.Expires = time.Unix(int64(options.Expires), 0)
		}
		opts.Volatile.EncryptionParameters = newStorjEncryptionParameters(options.EncryptionParameters)
		opts.Volatile.RedundancyScheme = newStorjRedundancyScheme(options.RedundancyScheme)
	}

	writer, err := bucket.lib.NewWriter(scope.ctx, path, opts)
	if err != nil {
		return nil, safeError(err)
	}
	return &Writer{scope, writer}, nil
}

// Write writes data.length bytes from data to the underlying data stream.
func (w *Writer) Write(data []byte, offset, length int32) (int32, error) {
	// in Java byte array size is max int32
	n, err := w.writer.Write(data[offset : offset+length])
	return int32(n), safeError(err)
}

// Cancel cancels writing operation
func (w *Writer) Cancel() {
	w.cancel()
}

// Close closes writer
func (w *Writer) Close() error {
	defer w.cancel()
	return safeError(w.writer.Close())
}

// ReaderOptions options for reading
type ReaderOptions struct {
}

// Reader reader for downloading object
type Reader struct {
	scope
	readError error
	reader    io.ReadCloser
}

// NewReader returns new reader for downloading object.
func (bucket *Bucket) NewReader(path storj.Path, options *ReaderOptions) (*Reader, error) {
	scope := bucket.scope.child()

	reader, err := bucket.lib.Download(scope.ctx, path)
	if err != nil {
		return nil, safeError(err)
	}
	return &Reader{
		scope:  scope,
		reader: reader,
	}, nil
}

// NewRangeReader returns new reader for downloading a range from the object.
func (bucket *Bucket) NewRangeReader(path storj.Path, start, limit int64, options *ReaderOptions) (*Reader, error) {
	scope := bucket.scope.child()

	reader, err := bucket.lib.DownloadRange(scope.ctx, path, start, limit)
	if err != nil {
		return nil, safeError(err)
	}
	return &Reader{
		scope:  scope,
		reader: reader,
	}, nil
}

// Read reads data into byte array
func (r *Reader) Read(data []byte, offset, length int32) (n int32, err error) {
	if r.readError != nil {
		err = r.readError
	} else {
		var read int
		read, err = r.reader.Read(data[offset : offset+length])
		n = int32(read)
	}

	if n > 0 && err != nil {
		r.readError = err
		err = nil
	}

	if err == io.EOF {
		return -1, nil
	}
	return n, safeError(err)
}

// Cancel cancels read operation
func (r *Reader) Cancel() {
	r.cancel()
}

// Close closes reader
func (r *Reader) Close() error {
	defer r.cancel()
	return safeError(r.reader.Close())
}
