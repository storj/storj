// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
	"crypto/sha256"
	"flag"
	"io"
	"io/ioutil"
	"strconv"
	"time"

	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/vivint/infectious"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/ranger"
)

var (
	pieceBlockSize = flag.Int("piece_block_size", 4*1024, "block size of pieces")
	key            = flag.String("key", "a key", "the secret key")
	rsk            = flag.Int("required", 5, "rs required")
	rsn            = flag.Int("total", 20, "rs total")
)

var nodes = []string{
	"35.232.47.152:4242",
	"35.226.188.0:4242",
	"35.202.182.77:4242",
	"35.232.88.171:4242",
	"35.225.50.137:4242",
	"130.211.168.182:4242",
	"104.197.5.134:4242",
	"35.192.126.41:4242",
	"35.193.196.52:4242",
	"130.211.203.52:4242",
	"35.224.122.76:4242",
	"104.198.233.25:4242",
	"35.226.21.152:4242",
	"130.211.221.239:4242",
	"35.202.144.175:4242",
	"130.211.190.250:4242",
	"35.194.56.141:4242",
	"35.192.108.107:4242",
	"35.202.91.191:4242",
	"35.192.7.33:4242",
}

const hardcodedPieceID = "zKNZHsYbbnnp082trbWQ67r7R6CyNtRuzvnwcWE\\="

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
	panic("TODO")
}

func (s *storjObjects) DeleteObject(ctx context.Context, bucket,
	object string) error {
	panic("TODO")
}

func (s *storjObjects) GetBucketInfo(ctx context.Context, bucket string) (
	bucketInfo minio.BucketInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) GetObject(ctx context.Context, bucket, object string,
	startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	encKey := sha256.Sum256([]byte(*key))
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	var firstNonce [12]byte
	decrypter, err := eestream.NewAESGCMDecrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
	if err != nil {
		return err
	}
	errs := make(chan error, *rsn)
	conns := make([]*grpc.ClientConn, *rsn)
	for i := 0; i < *rsn; i++ {
		go func(i int) {
			conns[i], err = grpc.Dial(nodes[i], grpc.WithInsecure())
			errs <- err
		}(i)
	}
	for range conns {
		err := <-errs
		if err != nil {
			// TODO don't fail just because of failure with only one node
			return err
		}
	}
	defer func() {
		for i := 0; i < *rsn; i++ {
			defer conns[i].Close()
		}
	}()
	rrs := map[int]ranger.Ranger{}
	type rangerInfo struct {
		i   int
		rr  ranger.Ranger
		err error
	}
	rrch := make(chan rangerInfo, *rsn)
	for i := 0; i < *rsn; i++ {
		go func(i int) {
			cl := client.New(ctx, conns[i])
			rr, err := ranger.GRPCRanger(cl, hardcodedPieceID)
			rrch <- rangerInfo{i, rr, err}
		}(i)
	}
	for range conns {
		info := <-rrch
		if info.err != nil {
			// TODO don't fail just because of failure with only one node
			return info.err
		}
		rrs[info.i] = info.rr
	}
	rr, err := eestream.Decode(context.Background(), rrs, es, 4*1024*1024)
	if err != nil {
		return err
	}
	rr, err = eestream.Transform(rr, decrypter)
	if err != nil {
		return err
	}
	rr, err = eestream.UnpadSlow(rr)
	if err != nil {
		return err
	}
	if length == -1 {
		length = rr.Size()
	}
	r := rr.Range(startOffset, length)
	_, err = io.Copy(writer, r)
	return err
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket, object string) (objInfo minio.ObjectInfo, err error) {
	return minio.ObjectInfo{
		Bucket:      bucket,
		Name:        object,
		ModTime:     time.Now(),
		Size:        111180613,
		IsDir:       false,
		ContentType: "video/mp4",
	}, nil
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
	panic("TODO")
}

//encryptFile encrypts the uploaded files
func encryptFile(ctx context.Context, data io.ReadCloser) error {
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	encKey := sha256.Sum256([]byte(*key))
	var firstNonce [12]byte
	encrypter, err := eestream.NewAESGCMEncrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
	if err != nil {
		return err
	}
	readers, err := eestream.EncodeReader(context.Background(), eestream.TransformReader(
		eestream.PadReader(data, encrypter.InBlockSize()), encrypter, 0),
		es, 0, 0, 4*1024*1024)
	if err != nil {
		return err
	}
	errs := make(chan error, len(readers))
	for i := range readers {
		go func(i int) {
			conn, err := grpc.Dial(nodes[i], grpc.WithInsecure())
			if err != nil {
				errs <- err
				return
			}
			defer conn.Close()
			cl := client.New(ctx, conn)
			// TODO just for the demo - the id is hardcoded
			// w, err := cl.StorePieceRequest(pstore.DetermineID(), 0)
			w, err := cl.StorePieceRequest(hardcodedPieceID, 0)
			if err != nil {
				errs <- err
				return
			}
			_, err = io.Copy(w, readers[i])
			// Make sure writer is closed to avoid losing last bytes
			if err != nil {
				w.Close()
				errs <- err
				return
			}
			errs <- w.Close()
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
	err = encryptFile(ctx, ioutil.NopCloser(data))
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
