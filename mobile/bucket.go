package mobile

import (
	"fmt"
	"io"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

const (
	// EncUnspecified indicates no encryption suite has been selected.
	CipherSuite_EncUnspecified = byte(storj.EncUnspecified)
	// EncNull indicates use of the NULL cipher; that is, no encryption is
	// done. The ciphertext is equal to the plaintext.
	CipherSuite_EncNull = byte(storj.EncNull)
	// EncAESGCM indicates use of AES128-GCM encryption.
	CipherSuite_EncAESGCM = byte(storj.EncAESGCM)
	// EncSecretBox indicates use of XSalsa20-Poly1305 encryption, as provided
	// by the NaCl cryptography library under the name "Secretbox".
	CipherSuite_EncSecretBox = byte(storj.EncSecretBox)
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
	Created              string
	SegmentsSize         int64
	RedundancyScheme     RedundancyScheme
	PathCipher           byte
	EncryptionParameters EncryptionParameters
}

func newBucketInfo(bucket storj.Bucket) *BucketInfo {
	return &BucketInfo{
		Name:    bucket.Name,
		Created: bucket.Created.String(),
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

type BucketList struct {
	list storj.BucketList
}

func (bl *BucketList) More() bool {
	return bl.list.More
}

func (bl *BucketList) Length() int {
	return len(bl.list.Items)
}

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

// Close bucket
func (bucket *Bucket) Close() error {
	defer bucket.cancel()
	return bucket.lib.Close()
}

type WriterOptions struct {
}

func NewWriterOptions() *WriterOptions {
	return &WriterOptions{}
}

type Writer struct {
	scope
	writer io.WriteCloser
}

func (bucket *Bucket) NewWriter(path storj.Path, options *WriterOptions) (*Writer, error) {
	scope := bucket.scope.child()

	opts := &libuplink.UploadOptions{}
	writer, err := bucket.lib.NewWriter(scope.ctx, path, opts)
	if err != nil {
		return nil, err
	}
	return &Writer{scope, writer}, nil
}

func (w *Writer) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

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

func (bucket *Bucket) NewReader(path storj.Path, options *ReaderOptions) (*Reader, error) {
	scope := bucket.scope.child()

	reader, err := bucket.lib.NewReader(scope.ctx, path)
	if err != nil {
		return nil, err
	}
	return &Reader{scope, reader}, nil
}

func (r *Reader) Read(data []byte) (int, error) {
	return r.reader.Read(data)
}

func (r *Reader) Close() error {
	defer r.cancel()
	return r.reader.Close()
}
