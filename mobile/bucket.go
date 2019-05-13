// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package mobile

import (
	"fmt"
	"io"
	"time"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
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

	DirectionAfter   = byte(storj.After)
	DirectionForward = byte(storj.Forward)
)

type Bucket struct {
	Name string

	scope
	lib *libuplink.Bucket
}

type BucketAccess struct {
	PathEncryptionKey   []byte
	EncryptedPathPrefix storj.Path
}

type BucketInfo struct {
	Name                 string
	Created              int
	SegmentsSize         int64
	RedundancyScheme     RedundancyScheme
	PathCipher           byte
	EncryptionParameters EncryptionParameters
}

func newBucketInfo(bucket storj.Bucket) *BucketInfo {
	return &BucketInfo{
		Name:    bucket.Name,
		Created: int(bucket.Created.UTC().Unix()),
		RedundancyScheme: RedundancyScheme{
			Algorithm:      byte(bucket.RedundancyScheme.Algorithm),
			ShareSize:      bucket.RedundancyScheme.ShareSize,
			RequiredShares: bucket.RedundancyScheme.RequiredShares,
			RepairShares:   bucket.RedundancyScheme.RepairShares,
			OptimalShares:  bucket.RedundancyScheme.OptimalShares,
			TotalShares:    bucket.RedundancyScheme.TotalShares,
		},
		EncryptionParameters: EncryptionParameters{
			CipherSuite: byte(bucket.EncryptionParameters.CipherSuite),
			BlockSize:   bucket.EncryptionParameters.BlockSize,
		},
	}
}

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
// It is like EncryptionScheme, but uses the CipherSuite type instead of Cipher.
// EncryptionParameters is preferred for new uses.
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

// ListOptions lists objects
type ListOptions struct {
	Prefix    string
	Cursor    string // Cursor is relative to Prefix, full path is Prefix + Cursor
	Delimiter string
	Recursive bool
	Direction byte
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
		// opts.Delimiter = options.Delimiter
		opts.Recursive = options.Recursive
		opts.Limit = options.Limit
	}

	list, err := bucket.lib.ListObjects(scope.ctx, opts)
	if err != nil {
		return nil, err
	}
	return &ObjectList{list}, nil
}

// DeleteObject removes an object, if authorized.
func (bucket *Bucket) DeleteObject(objectName string) error {
	scope := bucket.scope.child()
	return bucket.lib.DeleteObject(scope.ctx, objectName)
}

// Close closes the Bucket session.
func (bucket *Bucket) Close() error {
	defer bucket.cancel()
	return bucket.lib.Close()
}

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

	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// EncryptionParameters determines the cipher suite to use for
		// the Object's data encryption. If not set, the Bucket's
		// defaults will be used.
		EncryptionParameters *EncryptionParameters

		// RedundancyScheme determines the Reed-Solomon and/or Forward
		// Error Correction encoding parameters to be used for this
		// Object.
		RedundancyScheme *RedundancyScheme
	}
}

func NewWriterOptions() *WriterOptions {
	return &WriterOptions{}
}

type Writer struct {
	scope
	writer io.WriteCloser
}

// NewWriter creates instance of Writer
func (bucket *Bucket) NewWriter(path storj.Path, options *WriterOptions) (*Writer, error) {
	scope := bucket.scope.child()

	opts := &libuplink.UploadOptions{}
	opts.ContentType = options.ContentType
	opts.Metadata = options.Metadata
	if options.Expires != 0 {
		opts.Expires = time.Unix(int64(options.Expires), 0)
	}
	// opts.Volatile.EncryptionParameters =  options.Volatile.EncryptionParameters

	opts.Volatile.RedundancyScheme = newStorjRedundancyScheme(options.Volatile.RedundancyScheme)

	writer, err := bucket.lib.NewWriter(scope.ctx, path, opts)
	if err != nil {
		return nil, err
	}
	return &Writer{scope, writer}, nil
}

// Write writes data.length bytes from data to the underlying data stream.
func (w *Writer) Write(data []byte) (int32, error) {
	// in Java byte array size is max int32
	n, err := w.writer.Write(data)
	return int32(n), err
}

// Close closes writer
func (w *Writer) Close() error {
	defer w.cancel()
	return w.writer.Close()
}

type ReaderOptions struct {
}

type Reader struct {
	scope
	reader interface {
		io.Reader
		io.Seeker
		io.Closer
	}
}

// NewReader returns new reader for downloading object
func (bucket *Bucket) NewReader(path storj.Path, options *ReaderOptions) (*Reader, error) {
	scope := bucket.scope.child()

	reader, err := bucket.lib.NewReader(scope.ctx, path)
	if err != nil {
		return nil, err
	}
	return &Reader{scope, reader}, nil
}

func (r *Reader) Read(data []byte) (int32, error) {
	// TODO add validation for int vs int32
	n, err := r.reader.Read(data)
	if err == io.EOF {
		return -1, nil
	}
	return int32(n), err
}

func (r *Reader) Close() error {
	defer r.cancel()
	return r.reader.Close()
}
