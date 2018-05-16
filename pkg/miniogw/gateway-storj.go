// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

/*
#cgo CFLAGS: -I .
#cgo LDFLAGS: -L . -lstorj
#cgo LDFLAGS: -L /usr/lib -lcurl -lnettle -ljson-c -luv -lm
#include "storj.h"
#include <assert.h>

void getbucketscallback(uv_work_t *work_req, int status); // Forward declaration.
void storj_uv_run_cgo(storj_env_t *env);
int size_of_buckets_struct(void);
storj_bucket_meta_t *bucket_index(storj_bucket_meta_t *array, int index);
*/
import "C"
import (
	"context"
	"fmt"
	"io"
	"time"
	"unsafe"

	"github.com/minio/cli"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
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

// S3CliAPI contains parameters for accessing the Storj network
type S3CliAPI struct {
	env        *C.storj_env_t
	bucketInfo []minio.Bucket
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

	for i := uint(1); i < uint(req.total_buckets); i++ {
		bucket := C.bucket_index(req.buckets, C.int(i))

		//gS3CliApi.buckets = append(gS3CliApi.buckets, C.GoString(bucket.name))
		gS3CliAPI.bucketInfo = append(gS3CliAPI.bucketInfo, minio.Bucket{Name: C.GoString(bucket.name), CreationDate: C.GoString(bucket.created)})
		//gS3CliApi.created = append(gS3CliApi.created, C.GoString(bucket.created))

		fmt.Printf("ID: %s \tDecrypted: %t \tCreated: %s \tName: %s\n",
			C.GoString(bucket.id), bucket.decrypted,
			C.GoString(bucket.created), C.GoString(bucket.name))
	}

	C.storj_free_get_buckets_request(req)
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
		t, err := time.Parse(time.RFC3339, bi.CreationDate)
		if err != nil {
			t = time.Now()
		}
		b[i] = minio.BucketInfo{
			Name:    bi.Name,
			Created: t,
		}
	}

	return b, err
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	return minio.ListObjectsInfo{
		IsTruncated: false,
		Objects: []minio.ObjectInfo{{
			Bucket:      "test-bucket",
			Name:        "test-file",
			ModTime:     time.Now(),
			Size:        0,
			IsDir:       false,
			ContentType: "application/octet-stream",
		}},
	}, nil
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) error {
	panic("TODO")
}

func (s *storjObjects) PutObject(ctx context.Context, bucket, object string,
	data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo,
	err error) {
	panic("TODO")
}

func (s *storjObjects) Shutdown(context.Context) error {
	panic("TODO")
}

func (s *storjObjects) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}
