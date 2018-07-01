// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package objects

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/vivint/infectious"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/ranger"
)

var (
	pieceBlockSize = flag.Int("piece_block_size", 4*1024, "block size of pieces")
	key            = flag.String("key", "a key", "the secret key")
	rsk            = flag.Int("required", 20, "rs required")
	rsn            = flag.Int("total", 40, "rs total")
)

//Objects structure
type Objects struct {
	//segStore    segments.SegmentStore
	//streamStore streams.StreamStore
}

//Meta structure
type Meta struct {
	//Modified   time.Time
	Expiration time.Time
	Data       []byte
	// ObjectInfo - represents object metadata.
	// Name of the bucket.
	Bucket string

	// Name of the object.
	Name string

	// Date and time when the object was last modified.
	ModTime time.Time

	// Total object size.
	Size int64

	// IsDir indicates if the object is prefix.
	IsDir bool

	// Hex encoded unique entity tag of the object.
	ETag string

	// A standard MIME type describing the format of the object.
	ContentType string

	// Specifies what content encodings have been applied to the object and thus
	// what decoding mechanisms must be applied to obtain the object referenced
	// by the Content-Type header field.
	ContentEncoding string

	// Specify object storage class
	StorageClass string

	// User-Defined metadata
	UserDefined map[string]string

	// Date and time when the object was last accessed.
	AccTime time.Time
}

// func NewObjects(store streams.StreamStore) ObjectStore {
// 	panic("TODO")
// }
type recordingReader struct {
	data   io.Reader
	amount int64
}

func (r *recordingReader) Read(p []byte) (n int, err error) {
	n, err = r.data.Read(p)
	r.amount += int64(n)
	return n, err
}

//encryptFile encrypts the uploaded files
func encryptFile(data io.Reader, objPath string) (size int64, err error) {
	dir := os.TempDir()
	dir = filepath.Join(dir, "gateway", objPath)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return 0, err
	}
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return 0, err
	}
	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	encKey := sha256.Sum256([]byte(*key))
	var firstNonce [12]byte
	encrypter, err := eestream.NewAESGCMEncrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
	if err != nil {
		return 0, err
	}
	recorder := &recordingReader{
		data: data,
	}
	readers, err := eestream.EncodeReader(context.Background(), eestream.TransformReader(
		eestream.PadReader(ioutil.NopCloser(recorder), encrypter.InBlockSize()), encrypter, 0),
		es, 0, 0, 4*1024*1024)
	if err != nil {
		return 0, err
	}
	errs := make(chan error, len(readers))
	for i := range readers {
		go func(i int) {
			fh, err := os.Create(
				filepath.Join(dir, fmt.Sprintf("%d.piece", i)))
			if err != nil {
				errs <- err
				return
			}
			defer fh.Close()
			_, err = io.Copy(fh, readers[i])
			errs <- err
		}(i)
	}
	for range readers {
		err := <-errs
		if err != nil {
			return 0, err
		}
	}
	return recorder.amount, nil
}

//PutObject interface method
func (o *Objects) PutObject(ctx context.Context, objpath string, data io.Reader, metadata []byte, expiration time.Time) (size int64, err error) {
	defer mon.Task()(&ctx)(&err)
	wsize, err := encryptFile(data, objpath)
	return wsize, err
}

//GetObject interface method
func (o *Objects) GetObject(ctx context.Context, objpath string) (r ranger.Ranger, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	m = Meta{
		Name: "GetObject",
	}
	return nil, m, nil
}

//DeleteObject interface method
func (o *Objects) DeleteObject(ctx context.Context, objpath string) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//ListObjects interface method
func (o *Objects) ListObjects(ctx context.Context, startingPath, endingPath string) (objpaths []string, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//SetXAttr interface method
func (o *Objects) SetXAttr(ctx context.Context, objpath, xattr string, data io.Reader, metadata []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//GetXAttr interface method
func (o *Objects) GetXAttr(ctx context.Context, objpath, xattr string) (r ranger.Ranger, m Meta, err error) {
	defer mon.Task()(&ctx)(&err)
	getXAttrPath := path.Join("xattr", objpath, xattr)

	/* TODO@ASK clean up the below line */
	fmt.Println(getXAttrPath)
	panic("TODO")
}

//DeleteXAttr interface method
func (o *Objects) DeleteXAttr(ctx context.Context, path, xattr string) (err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}

//ListXAttrs interface method
func (o *Objects) ListXAttrs(ctx context.Context, path, startingXAttr, endingXAttr string) (xattrs []string, truncated bool, err error) {
	defer mon.Task()(&ctx)(&err)
	panic("TODO")
}
