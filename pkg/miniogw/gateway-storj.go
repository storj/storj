// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

/*
#cgo CFLAGS: -I .
#cgo LDFLAGS: -L . -lstorj
#cgo LDFLAGS: -L /usr/lib -lcurl -lnettle -ljson-c -luv -lm
#include "storj.h"

void getbucketscallback(uv_work_t *work_req, int status); // Forward declaration.
void listfilescallback(uv_work_t *work_req, int status);  // Forward declaration.
void storj_uv_run_cgo(storj_env_t *env);
int size_of_buckets_struct(void);
storj_bucket_meta_t *bucket_index(storj_bucket_meta_t *array, int index);
storj_file_meta_t *file_index(storj_file_meta_t *array, int index);
int upload_file(storj_env_t *env, char *bucket_id, const char *file_path, char *file_name, void *handle);
void file_open_test(void);
*/
import "C"
import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

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

// gGoEnvT global storj env structure declaration
var gGoEnvT *C.storj_env_t

// Env contains parameters for accessing the Storj network
// type Env struct {
// 	URL      string
// 	User     string
// 	Password string
// 	Mnemonic string
// }

//S3Bucket structure
type S3Bucket struct {
	bucket   minio.Bucket
	bucketID string
}

// S3CliAPI contains parameters for accessing the Storj network
type S3CliAPI struct {
	env        *C.storj_env_t
	bucketInfo []S3Bucket
	fileInfo   minio.ListObjectsInfo
}

// gS3CliApi global S3 interface structure
var gS3CliAPI S3CliAPI

//export getbucketscallback
func getbucketscallback(workreq *C.uv_work_t, status C.int) {
	fmt.Printf("Go.getbucketscallback(): called with status = %d\n", status)

	var req *C.get_buckets_request_t
	req = (*C.get_buckets_request_t)(unsafe.Pointer(workreq.data))

	if req.status_code == 401 {
		fmt.Printf("Invalid user credentials.\n")
	} else if req.status_code == 403 {
		fmt.Printf("Forbidden, user not active.\n")
	} else if req.status_code != 200 && req.status_code != 304 {
		fmt.Printf("Request failed with status code: %d\n", req.status_code)
	} else if req.total_buckets == 0 {
		fmt.Printf("No buckets.\n")
	}

	/* clear the bucket */
	gS3CliAPI.bucketInfo = gS3CliAPI.bucketInfo[:0]
	for i := uint(0); i < uint(req.total_buckets); i++ {
		bucket := C.bucket_index(req.buckets, C.int(i))

		gS3CliAPI.bucketInfo = append(gS3CliAPI.bucketInfo,
			S3Bucket{
				bucket: minio.Bucket{
					Name:         C.GoString(bucket.name),
					CreationDate: C.GoString(bucket.created)},
				bucketID: C.GoString(bucket.id)})

		fmt.Printf("ID: %s \tDecrypted: %t \tCreated: %s \tName: %s\n",
			C.GoString(bucket.id), bucket.decrypted,
			C.GoString(bucket.created), C.GoString(bucket.name))
	}

	fmt.Println("bucketInfo= ", gS3CliAPI.bucketInfo)
	C.storj_free_get_buckets_request(req)
	C.free(unsafe.Pointer(workreq))
}

//export listfilescallback
func listfilescallback(workreq *C.uv_work_t, status C.int) {
	fmt.Printf("Go.listfilescallback(): called with status = %d\n", status)

	var req *C.list_files_request_t
	req = (*C.list_files_request_t)(unsafe.Pointer(workreq.data))

	if req.status_code == 404 {
		fmt.Printf("Bucket id [%s] does not exist\n", C.GoString(req.bucket_id))
		goto cleanup
	} else if req.status_code == 400 {
		fmt.Printf("Bucket id [%s] is invalid\n", C.GoString(req.bucket_id))
		goto cleanup
	} else if req.status_code == 401 {
		fmt.Printf("Invalid user credentials.\n")
		goto cleanup
	} else if req.status_code == 403 {
		fmt.Printf("Forbidden, user not active.\n")
		goto cleanup
	} else if req.status_code != 200 {
		fmt.Printf("Request failed with status code: %d\n", C.int(req.status_code))
	}

	if req.total_files == 0 {
		fmt.Printf("No files for bucket.\n")
		goto cleanup
	}

	/* clear the file info */
	gS3CliAPI.fileInfo.Objects = gS3CliAPI.fileInfo.Objects[:0]
	for i := uint(0); i < uint(req.total_files); i++ {
		file := C.file_index(req.files, C.int(i))
		t, err := time.Parse(time.RFC3339, C.GoString(file.created))
		if err != nil {
			t = time.Now()
		}
		gS3CliAPI.fileInfo.Objects = append(gS3CliAPI.fileInfo.Objects,
			minio.ObjectInfo{
				Name:        C.GoString(file.filename),
				ModTime:     t,
				Size:        int64(C.int(file.size)),
				ContentType: C.GoString(file.mimetype),
				IsDir:       false})

		fmt.Printf("ID: %s \tSize: %d \tDecrypted: %t \tType: %s \tCreated: %s \tName: %s\n",
			C.GoString(file.id),
			C.int(file.size),
			file.decrypted,
			C.GoString(file.mimetype),
			C.GoString(file.created),
			C.GoString(file.filename))
	}

cleanup:

	fmt.Printf("hello clean up \n")
	C.storj_free_list_files_request(req)
	C.free(unsafe.Pointer(workreq))
}

// NewEnv creates new Env struct with default values
// func NewEnv() Env {
// 	return Env{
// 		URL:      "https://api.storj.io", //viper.GetString("bridge"),
// 		User:     "kishore@storj.io",     //viper.GetString("bridge-user"),
// 		Password: sha256Sum("xxxxxxx"),
// 		Mnemonic: "surface excess rude either pink bone pact ready what ability current plug",
// 	}
// }

func init() {
	minio.RegisterGatewayCommand(cli.Command{
		Name:            "storj",
		Usage:           "Storj",
		Action:          storjGatewayMain,
		HideHelpCommand: true,
	})

	gGoEnvT = C.storj_go_init()

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
	//Env
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
	x := C.storj_util_timestamp()
	fmt.Println("STORJ LIST BUCKETS COMMAND ", x)

	fmt.Printf("Go.main(): calling C function with callback to us\n")
	//C.some_c_func((C.callback_fcn)(unsafe.Pointer(C.callOnMeGo_cgo)))
	//C.storj_bridge_get_buckets(gGoEnvT, nil, (C.uv_after_work_cb)(unsafe.Pointer(C.getbucketscallback_cgo)))
	response := C.storj_bridge_get_buckets((*C.storj_env_t)(gGoEnvT), nil, (C.uv_after_work_cb)(unsafe.Pointer(C.getbucketscallback)))

	C.storj_uv_run_cgo(gGoEnvT)
	fmt.Printf("Go.main(): calling C function with callback, returned response = %d\n", response)

	b := make([]minio.BucketInfo, len(gS3CliAPI.bucketInfo))
	for i, bi := range gS3CliAPI.bucketInfo {
		t, err := time.Parse(time.RFC3339, bi.bucket.CreationDate)
		if err != nil {
			t = time.Now()
		}
		b[i] = minio.BucketInfo{
			Name:    bi.bucket.Name,
			Created: t,
		}
	}

	return b, nil
}

//GetBucketID returns the corresponding bucketID
func GetBucketID(bucketName string) (bucketID string) {
	var bktName string
	/* @TODO: Handle Zero files in a bucket name */
	for i, v := range gS3CliAPI.bucketInfo {
		bucketID = v.bucketID
		bktName = v.bucket.Name
		fmt.Printf("ID: %s \tName: %s\n", bucketID, bucketName)
		ret := strings.Compare(bktName, bucketName)
		if ret == 0x00 {
			bucketID = v.bucketID
			bucketName = v.bucket.Name
			break
		}
		/* @TODO: Invalid bucket name handle here... */
		if i == (len(gS3CliAPI.bucketInfo) - 1) {
			fmt.Printf("Invalid bucket name \n")
			bucketID = ""
		}
	}

	fmt.Printf("gS3CliAPI.bucketID = %s \n", bucketID)
	fmt.Printf("gS3CliAPI.bucketInfo[n].Name = %s\n", bucketName)
	return bucketID
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {

	var bucketID string
	var bucketName string
	/* @TODO: Handle Zero files in a bucket name */
	for i, v := range gS3CliAPI.bucketInfo {
		bucketID = v.bucketID
		bucketName = v.bucket.Name
		fmt.Printf("ID: %s \tName: %s\n", bucketID, bucketName)
		ret := strings.Compare(bucketName, bucket)
		if ret == 0x00 {
			bucketID = v.bucketID
			bucketName = v.bucket.Name
			break
		}
		/* @TODO: Invalid bucket name handle here... */
		if i == (len(gS3CliAPI.bucketInfo) - 1) {
			fmt.Printf("Invalid bucket name \n")
		}
	}

	fmt.Printf("gS3CliAPI.bucketID = %s \n", bucketID)
	fmt.Printf("gS3CliAPI.bucketInfo[n].Name = %s\n", bucketName)

	response := C.storj_bridge_list_files((*C.storj_env_t)(gGoEnvT), C.CString(bucketID), nil, (C.uv_after_work_cb)(unsafe.Pointer(C.listfilescallback)))
	C.storj_uv_run_cgo(gGoEnvT)
	fmt.Printf("Go.main(): calling C function with callback, returned response = %d\n", response)

	b := make([]minio.ObjectInfo, len(gS3CliAPI.fileInfo.Objects))
	/* @TODO: Handle Zero files in a bucket name */
	for i, bi := range gS3CliAPI.fileInfo.Objects {
		b[i] = minio.ObjectInfo{
			Bucket:      bucketName,
			Name:        bi.Name,
			ModTime:     bi.ModTime,
			Size:        bi.Size,
			IsDir:       bi.IsDir,
			ContentType: bi.ContentType,
		}
	}

	//return b, nil
	return minio.ListObjectsInfo{
		IsTruncated: false,
		Objects:     b,
	}, nil
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) error {
	panic("TODO")
}

// Main is the exported CLI executable function
func Main(data io.ReadCloser, blockSize uint) error {
	dir := "gateway"
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
	Main(writer, uint(wsize))
	bucketID := GetBucketID(bucket)
	fmt.Println("BucketID = ", bucketID)
	//C.file_open_test()
	// if bucketID != "" {
	// 	_, fileName := path.Split("/Users/kishor/Downloads/upload_testfile.txt")
	// 	response := C.upload_file((*C.storj_env_t)(gGoEnvT), C.CString(bucketID), C.CString("/Users/kishore/Downloads/upload_testfile.txt"), C.CString(fileName), nil)
	// 	C.storj_uv_run_cgo((*C.storj_env_t)(gGoEnvT))
	// 	fmt.Printf("Go.main(): calling C function with callback, returned response = %d\n", response)
	// }

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
