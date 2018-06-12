// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/protos/overlay"
)

var (
	pieceBlockSize = flag.Int("piece_block_size", 4*1024, "block size of pieces")
	key            = flag.String("key", "a key", "the secret key")
	rsk            = flag.Int("required", 5, "rs required")
	rsn            = flag.Int("total", 20, "rs total")
	rsm            = flag.Int("minimum", 0, "rs minimum safe")
	rso            = flag.Int("optimal", 0, "rs optimal safe")

	mon   = monkit.Package()
	Error = errs.Class("error")
)

func init() {
	minio.RegisterGatewayCommand(cli.Command{
		Name:            "storj",
		Usage:           "Storj",
		Action:          storjGatewayMain,
		HideHelpCommand: true,
	})
}

// getBuckets returns the buckets list
func (s *storjObjects) getBuckets() (buckets []minio.BucketInfo, err error) {
	buckets = make([]minio.BucketInfo, len(s.storj.bucketlist))
	for i, bi := range s.storj.bucketlist {
		buckets[i] = minio.BucketInfo{
			Name:    bi.bucket.Name,
			Created: bi.bucket.Created,
		}
	}
	return buckets, nil
}

// uploadFile function handles to add the uploaded file to the bucket's file list structure
func (s *storjObjects) uploadFile(bucket, object string, filesize int64, metadata map[string]string) (result minio.ListObjectsInfo, err error) {
	var fl []minio.ObjectInfo
	for i, v := range s.storj.bucketlist {
		// bucket string comparision
		if v.bucket.Name == bucket {
			/* append the file to the filelist */
			s.storj.bucketlist[i].filelist.file.Objects = append(
				s.storj.bucketlist[i].filelist.file.Objects,
				minio.ObjectInfo{
					Bucket:      bucket,
					Name:        object,
					ModTime:     time.Now(),
					Size:        filesize,
					IsDir:       false,
					ContentType: "video/mp4",
				},
			)
			/* populate the filelist */
			f := make([]minio.ObjectInfo, len(s.storj.bucketlist[i].filelist.file.Objects))
			for j, fi := range s.storj.bucketlist[i].filelist.file.Objects {
				f[j] = minio.ObjectInfo{
					Bucket:      v.bucket.Name,
					Name:        fi.Name,
					ModTime:     fi.ModTime,
					Size:        fi.Size,
					IsDir:       fi.IsDir,
					ContentType: fi.ContentType,
				}
			}
			fl = f
			break
		}
	}
	result = minio.ListObjectsInfo{
		IsTruncated: false,
		Objects:     fl,
	}
	return result, nil
}

// getFiles returns the files list for a bucket
func (s *storjObjects) getFiles(bucket string) (result minio.ListObjectsInfo, err error) {
	var fl []minio.ObjectInfo
	for i, v := range s.storj.bucketlist {
		if v.bucket.Name == bucket {
			/* populate the filelist */
			f := make([]minio.ObjectInfo, len(s.storj.bucketlist[i].filelist.file.Objects))
			for j, fi := range s.storj.bucketlist[i].filelist.file.Objects {
				f[j] = minio.ObjectInfo{
					Bucket:      v.bucket.Name,
					Name:        fi.Name,
					ModTime:     fi.ModTime,
					Size:        fi.Size,
					IsDir:       fi.IsDir,
					ContentType: fi.ContentType,
				}
			}
			fl = f
			break
		}
	}
	result = minio.ListObjectsInfo{
		IsTruncated: false,
		Objects:     fl,
	}
	return result, nil
}

// deleteFile returns the files list for a bucket
func (s *storjObjects) deleteFile(bucket, object string) (err error) {
	for i, v := range s.storj.bucketlist {
		k := 0
		if v.bucket.Name == bucket {
			for j := 0; j < len(s.storj.bucketlist[i].filelist.file.Objects); j++ {
				fi := s.storj.bucketlist[i].filelist.file.Objects[j].Name
				fmt.Println("fi=", fi)
				if fi != object {
					s.storj.bucketlist[i].filelist.file.Objects[k].Name = fi
					k++
				}
			}
			s.storj.bucketlist[i].filelist.file.Objects = s.storj.bucketlist[i].filelist.file.Objects[:k]
		}
	}
	return nil
}

// removeBucket returns the files list for a bucket
func (s *storjObjects) removeBucket(bucket string) (err error) {
	k := 0
	for i, v := range s.storj.bucketlist {
		bi := s.storj.bucketlist[i].bucket.Name
		fmt.Println("bi=", bi)
		if v.bucket.Name != bucket {
			s.storj.bucketlist[k] = v
			k++
		}
	}
	s.storj.bucketlist = s.storj.bucketlist[:k]
	return nil
}

// addBucket returns the files list for a bucket
func (s *storjObjects) addBucket(bucket string) (err error) {
	s.storj.bucketlist = append(s.storj.bucketlist,
		S3Bucket{
			minio.BucketInfo{
				Name:    bucket,
				Created: time.Now(),
			},
			S3FileList{
				minio.ListObjectsInfo{},
			},
		},
	)
	return nil
}

func storjGatewayMain(ctx *cli.Context) {
	s := &Storj{}
	s.createSampleBucketList()
	minio.StartGateway(ctx, s)
}

//S3Bucket structure
type S3Bucket struct {
	bucket   minio.BucketInfo
	filelist S3FileList
}

//S3FileList structure
type S3FileList struct {
	file minio.ListObjectsInfo
}

// Storj is the implementation of a minio cmd.Gateway
type Storj struct {
	bucketlist []S3Bucket
}

// Name implements cmd.Gateway
func (s *Storj) Name() string {
	return "storj"
}

// NewGatewayLayer implements cmd.Gateway
func (s *Storj) NewGatewayLayer(creds auth.Credentials) (
	minio.ObjectLayer, error) {
	return &storjObjects{storj: s}, nil
}

// Production implements cmd.Gateway
func (s *Storj) Production() bool {
	return false
}

//createSampleBucketList function initializes sample buckets and files in each bucket
func (s *Storj) createSampleBucketList() {
	s.bucketlist = make([]S3Bucket, 10)
	for i := range s.bucketlist {
		s.bucketlist[i].bucket.Name = "TestBucket" + strconv.Itoa(i+1)
		s.bucketlist[i].bucket.Created = time.Now()
		s.bucketlist[i].filelist.file.IsTruncated = false
		s.bucketlist[i].filelist.file.Objects = make([]minio.ObjectInfo, 0x0A)
		for j := range s.bucketlist[i].filelist.file.Objects {
			s.bucketlist[i].filelist.file.Objects[j].Bucket = s.bucketlist[i].bucket.Name
			s.bucketlist[i].filelist.file.Objects[j].Name = s.bucketlist[i].bucket.Name + "file" + strconv.Itoa(j+1)
			s.bucketlist[i].filelist.file.Objects[j].ModTime = time.Now()
			s.bucketlist[i].filelist.file.Objects[j].Size = 100
			s.bucketlist[i].filelist.file.Objects[j].ContentType = "application/octet-stream"
		}
	}
}

type storjObjects struct {
	minio.GatewayUnsupported
	TempDir string // Temporary storage location for file transfers.
	storj   *Storj
}

func (s *storjObjects) DeleteBucket(ctx context.Context, bucket string) error {
	return s.removeBucket(bucket)
}

func (s *storjObjects) DeleteObject(ctx context.Context, bucket,
	object string) error {
	return s.deleteFile(bucket, object)
}

func (s *storjObjects) GetBucketInfo(ctx context.Context, bucket string) (
	bucketInfo minio.BucketInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) GetObject(ctx context.Context, bucket, object string,
	startOffset int64, length int64, writer io.Writer, etag string) (err error) {

	panic("TODO")
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket,
	object string) (objInfo minio.ObjectInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) ListBuckets(ctx context.Context) (
	buckets []minio.BucketInfo, err error) {
	return s.getBuckets()
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	return s.getFiles(bucket)
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) error {
	return s.addBucket(bucket)
}

//encryptFile encrypts the uploaded files
func encryptFile(ctx context.Context, data *hash.Reader, bucket, object string) (err error) {
	defer mon.Task()(&ctx)(&err)

	dir := os.TempDir()
	dir = filepath.Join(dir, "gateway", bucket, object)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}

	/* create a unique fileID */
	netStateKey := []byte(*key)
	encryptedPath, err := paths.Encrypt([]string{bucket, object}, netStateKey)
	fmt.Println("encrypted path ", encryptedPath)
	decryptedPath, err := paths.Decrypt(encryptedPath, netStateKey)
	fmt.Println("decrypted path ", decryptedPath)

	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	encKey := sha256.Sum256([]byte(*key))
	var firstNonce [12]byte
	encrypter, err := eestream.NewAESGCMEncrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
	if err != nil {
		return err
	}
	readers, err := eestream.EncodeReader(context.Background(), eestream.TransformReader(
		eestream.PadReader(ioutil.NopCloser(data), encrypter.InBlockSize()), encrypter, 0),
		es, *rsm, *rso, 4*1024*1024)
	if err != nil {
		return Error.Wrap(err)
	}

	/* integrating with DHT for uploading to get the farmer's IP address */
	addr := "bootstrap.storj.io:7070"
	c, err := overlay.NewOverlayClient(addr)
	if err != nil {
		return Error.Wrap(err)
	}

	/* TODO: get the space by sizeof(reader)*#of readers */
	r, err := c.Choose(context.Background(), int(20), int(100), int(100))
	if err != nil {
		return Error.Wrap(err)
	}
	fmt.Printf("r %#v\n", r)
	//pieceId := pstore.DetermineID()
	// r is your nodes
	var remotePieces []*pb.RemotePiece
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
			return err
		}
	}
	return nil
}

func (s *storjObjects) PutObject(ctx context.Context, bucket, object string,
	data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo,
	err error) {
	defer mon.Task()(&ctx)(&err)
	err = encryptFile(ctx, data, bucket, object)
	if err == nil {
		s.uploadFile(bucket, object, data.Size(), metadata)
	}
	return minio.ObjectInfo{
		Name:    object,
		Bucket:  bucket,
		ModTime: time.Now(),
		Size:    data.Size(),
		ETag:    minio.GenETag(),
	}, err
}

func (s *storjObjects) Shutdown(context.Context) error {
	panic("TODO")
}

func (s *storjObjects) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}
