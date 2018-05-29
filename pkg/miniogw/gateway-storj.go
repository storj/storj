// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/minio/cli"
	"github.com/vivint/infectious"
	"storj.io/storj/pkg/eestream"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
)

var (
	pieceBlockSize = flag.Int("piece_block_size", 4*1024, "block size of pieces")
	key            = flag.String("key", "a key", "the secret key")
	rsk            = flag.Int("required", 20, "rs required")
	rsn            = flag.Int("total", 40, "rs total")
)

// S3CliAPI contains parameters for accessing the Storj network
type S3CliAPI struct {
	totalBuckets int
	bucketlist   []S3BucketList
}

//S3BucketList structure
type S3BucketList struct {
	bucket   minio.BucketInfo
	filelist S3FileList
}

//S3FileList structure
type S3FileList struct {
	totalFiles int
	file       minio.ListObjectsInfo
}

// gS3Gateway global S3 interface structure
var gS3CliAPI S3CliAPI

//createDummyBucketList function initializes sample buckets and files in each bucket
func createDummyBucketList() {
	gS3CliAPI.bucketlist = gS3CliAPI.bucketlist[:0]
	for i := 0x00; i < 0x0A; i++ {
		gS3CliAPI.bucketlist = append(gS3CliAPI.bucketlist,
			S3BucketList{
				bucket: minio.BucketInfo{
					Name:    "TestBucket" + strconv.Itoa(i+1),
					Created: time.Now(),
				},
			},
		)
	} /* end of for loop */

	for i := 0x00; i < 0x0A; i++ {
		gS3CliAPI.bucketlist[i].filelist.file.IsTruncated = false
		gS3CliAPI.bucketlist[i].filelist.file.Objects = gS3CliAPI.bucketlist[i].filelist.file.Objects[:0]
		for j := 0x00; j < 0x0A; j++ {
			gS3CliAPI.bucketlist[i].filelist.file.Objects = append(
				gS3CliAPI.bucketlist[i].filelist.file.Objects,
				minio.ObjectInfo{
					Bucket:      gS3CliAPI.bucketlist[i].bucket.Name,
					Name:        gS3CliAPI.bucketlist[i].bucket.Name + "file" + strconv.Itoa(j+1),
					ModTime:     time.Now(),
					Size:        int64(100 + i),
					ContentType: "application/octet-stream",
				},
			)
		}
	}
	gS3CliAPI.totalBuckets = len(gS3CliAPI.bucketlist)
	fmt.Println("bucket name = ", gS3CliAPI.bucketlist)
}

//createDummyBucketList function initializes sample buckets and files in each bucket
// func createDummyBucketList() {
// 	gS3CliAPI.bucketlist = make([]S3BucketList, 0x0A)
// 	gS3CliAPI.totalBuckets = len(gS3CliAPI.bucketlist)
// 	//for i := 0x00; i < 0x0A; i++ {
// 	for i := range gS3CliAPI.bucketlist {
// 		gS3CliAPI.bucketlist[i].bucket.Name = "TestBucket#" + strconv.Itoa(i+1)
// 		gS3CliAPI.bucketlist[i].bucket.Created = time.Now()
// 		gS3CliAPI.bucketlist[i].filelist.file.IsTruncated = false
// 		gS3CliAPI.bucketlist[i].filelist.file.Objects = make([]minio.ObjectInfo, 0x0A)
// 		for j := range gS3CliAPI.bucketlist[i].filelist.file.Objects {
// 			gS3CliAPI.bucketlist[i].filelist.file.Objects[j].Bucket = gS3CliAPI.bucketlist[i].bucket.Name
// 			gS3CliAPI.bucketlist[i].filelist.file.Objects[j].Name = "file#" + strconv.Itoa(j+1)
// 			gS3CliAPI.bucketlist[i].filelist.file.Objects[j].ModTime = time.Now()
// 			gS3CliAPI.bucketlist[i].filelist.file.Objects[j].Size = 100
// 			gS3CliAPI.bucketlist[i].filelist.file.Objects[j].ContentType = "application/octet-stream"
// 		}
// 	} /* end of for loop */
// 	fmt.Println("bucket name = ", gS3CliAPI.bucketlist)
// }

func init() {
	minio.RegisterGatewayCommand(cli.Command{
		Name:            "storj",
		Usage:           "Storj",
		Action:          storjGatewayMain,
		HideHelpCommand: true,
	})

	// create dummy bucket list
	createDummyBucketList()
}

// getbucketscallback
func getbucketscallback() (buckets []minio.BucketInfo, err error) {
	buckets = make([]minio.BucketInfo, gS3CliAPI.totalBuckets)
	for i, bi := range gS3CliAPI.bucketlist {
		buckets[i] = minio.BucketInfo{
			Name:    bi.bucket.Name,
			Created: bi.bucket.Created,
		}
	}
	fmt.Println("buckets list =", buckets)
	return buckets, nil
}

// putobjectcallback
func putobjectcallback(bucket, object string, filesize int64, metadata map[string]string) (result minio.ListObjectsInfo, err error) {
	fmt.Printf("Go.putobjectcallback(): called \n")
	var bucketName string
	var fl []minio.ObjectInfo
	for i, v := range gS3CliAPI.bucketlist {
		bucketName = v.bucket.Name
		ret := strings.Compare(bucketName, bucket)
		if ret == 0x00 {
			/* append the file to the filelist */
			gS3CliAPI.bucketlist[i].filelist.file.Objects = append(
				gS3CliAPI.bucketlist[i].filelist.file.Objects,
				minio.ObjectInfo{
					Bucket:      bucket,
					Name:        object,
					ModTime:     time.Now(),
					Size:        filesize,
					IsDir:       false,
					ContentType: "application/octet-stream",
				},
			)
			/* populate the filelist */
			bucketName = v.bucket.Name
			f := make([]minio.ObjectInfo, len(gS3CliAPI.bucketlist[i].filelist.file.Objects))
			for j, fi := range gS3CliAPI.bucketlist[i].filelist.file.Objects {
				f[j] = minio.ObjectInfo{
					Bucket:      bucketName,
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
	fmt.Println("filelist: ", fl)
	return result, nil
}

// listfilescallback
func listfilescallback(bucket string) (result minio.ListObjectsInfo, err error) {
	fmt.Printf("Go.listfilescallback(): called \n")

	var bucketName string
	var fl []minio.ObjectInfo
	for i, v := range gS3CliAPI.bucketlist {
		bucketName = v.bucket.Name
		ret := strings.Compare(bucketName, bucket)
		if ret == 0x00 {
			/* populate the filelist */
			bucketName = v.bucket.Name
			f := make([]minio.ObjectInfo, len(gS3CliAPI.bucketlist[i].filelist.file.Objects))
			for j, fi := range gS3CliAPI.bucketlist[i].filelist.file.Objects {
				f[j] = minio.ObjectInfo{
					Bucket:      bucketName,
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
	fmt.Println("filelist: ", fl)
	return result, nil
}

func storjGatewayMain(ctx *cli.Context) {
	minio.StartGateway(ctx, &Storj{})
}

// Storj is the implementation of a minio cmd.Gateway
type Storj struct{}

// Name implements cmd.Gateway
func (s *Storj) Name() string {
	return "storj"
}

// NewGatewayLayer implements cmd.Gateway
func (s *Storj) NewGatewayLayer(creds auth.Credentials) (
	minio.ObjectLayer, error) {
	return &storjObjects{}, nil
}

// Production implements cmd.Gateway
func (s *Storj) Production() bool {
	return false
}

type storjObjects struct {
	minio.GatewayUnsupported
	TempDir string // Temporary storage location for file transfers.
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

	panic("TODO")
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket,
	object string) (objInfo minio.ObjectInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) ListBuckets(ctx context.Context) (
	buckets []minio.BucketInfo, err error) {
	buckets, err = getbucketscallback()
	return buckets, err
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	result, err = listfilescallback(bucket)
	return result, nil
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) error {
	panic("TODO")
}

//uploadcallback uploads the files
func uploadcallback(data io.ReadCloser, blockSize uint, bucket, object string) error {
	dir := "/tmp/gateway/" + bucket + "/" + object
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
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
	readers := eestream.EncodeReader(eestream.TransformReader(
		eestream.PadReader(data, encrypter.InBlockSize()), encrypter, 0), es)
	errs := make(chan error, len(readers))
	for i := range readers {
		go func(i int) {
			fh, err := os.Create(
				filepath.Join(dir, fmt.Sprintf("%s%d.piece", object, i)))
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
	srcFile := path.Join(s.TempDir, minio.MustGetUUID())
	writer, err := os.Create(srcFile)
	if err != nil {
		return objInfo, err
	}

	wsize, err := io.CopyN(writer, data, data.Size())
	if err != nil {
		os.Remove(srcFile)
		return objInfo, err
	}

	fmt.Printf("hello hello hello bucket = %s; object = %s wsize = %d\n ", bucket, object, wsize)
	fmt.Println(" data =", data)
	/* @TODO Attention: Need to handle the file size limit */

	uploadcallback(writer, uint(wsize), bucket, object)
	fmt.Println("Bucket = ", bucket)

	_, _ = putobjectcallback(bucket, object, wsize, metadata)
	return minio.ObjectInfo{
		Name:    object,
		Bucket:  bucket,
		ModTime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		Size:    wsize,
		ETag:    minio.GenETag(),
	}, nil
}

func (s *storjObjects) Shutdown(context.Context) error {
	panic("TODO")
}

func (s *storjObjects) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}
