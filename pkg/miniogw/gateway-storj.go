// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

/*
#cgo CFLAGS: -I .
#cgo LDFLAGS: -L . -lstorj
#cgo LDFLAGS: -L /usr/lib -lcurl -lnettle -ljson-c -luv -lm
#include "storj.h"

void getbucketscallback(uv_work_t *work_req, int status); // Forward declaration.
void storj_uv_run_cgo(storj_env_t *env);
*/
import "C"
import (
	"context"
	"fmt"
	"io"
	"time"
	"unsafe"

	"github.com/minio/cli"

	"github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
)

// global storj env structure declaration
var gGoEnvT *C.storj_env_t

// Env contains parameters for accessing the Storj network
type Env struct {
	URL      string
	User     string
	Password string
	Mnemonic string
}

//export getbucketscallback
func getbucketscallback(workreq *C.uv_work_t, status C.int) {
	fmt.Printf("Go.getbucketscallback(): called with status = %d\n", status)
}

// NewEnv creates new Env struct with default values
func NewEnv() Env {
	return Env{
		URL:      "https://api.storj.io", //viper.GetString("bridge"),
		User:     "kishore@storj.io",     //viper.GetString("bridge-user"),
		Password: sha256Sum("Njoy4ever"),
		Mnemonic: "surface excess rude either pink bone pact ready what ability current plug",
	}
}

func init() {
	cmd.RegisterGatewayCommand(cli.Command{
		Name:            "storj",
		Usage:           "Storj",
		Action:          storjGatewayMain,
		HideHelpCommand: true,
	})

	gGoEnvT = C.storj_go_init()

}

func storjGatewayMain(ctx *cli.Context) {
	cmd.StartGateway(ctx, &Storj{})
}

// Storj is the implementation of a minio cmd.Gateway
type Storj struct{}

// Name implements cmd.Gateway
func (s *Storj) Name() string {
	return "storj"
}

// NewGatewayLayer implements cmd.Gateway
func (s *Storj) NewGatewayLayer(creds auth.Credentials) (
	cmd.ObjectLayer, error) {
	return &storjObjects{}, nil
}

// Production implements cmd.Gateway
func (s *Storj) Production() bool {
	return false
}

type storjObjects struct {
	cmd.GatewayUnsupported
	Env
}

func (s *storjObjects) DeleteBucket(ctx context.Context, bucket string) error {
	panic("TODO")
}

func (s *storjObjects) DeleteObject(ctx context.Context, bucket,
	object string) error {
	panic("TODO")
}

func (s *storjObjects) GetBucketInfo(ctx context.Context, bucket string) (
	bucketInfo cmd.BucketInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) GetObject(ctx context.Context, bucket, object string,
	startOffset int64, length int64, writer io.Writer, etag string) (err error) {

	panic("TODO")
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket,
	object string) (objInfo cmd.ObjectInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) ListBuckets(ctx context.Context) (
	buckets []cmd.BucketInfo, err error) {
	x := C.storj_util_timestamp()
	fmt.Println("STORJ LIST BUCKETS COMMAND ", x)

	fmt.Printf("Go.main(): calling C function with callback to us\n")
	//C.some_c_func((C.callback_fcn)(unsafe.Pointer(C.callOnMeGo_cgo)))
	//C.storj_bridge_get_buckets(gGoEnvT, nil, (C.uv_after_work_cb)(unsafe.Pointer(C.getbucketscallback_cgo)))
	response := C.storj_bridge_get_buckets((*C.storj_env_t)(gGoEnvT), nil, (C.uv_after_work_cb)(unsafe.Pointer(C.getbucketscallback)))

	C.storj_uv_run_cgo(gGoEnvT)
	fmt.Printf("Go.main(): calling C function with callback, returned response = %d\n", response)

	return []cmd.BucketInfo{{
		Name:    "test-bucket",
		Created: time.Now(),
	}}, nil
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result cmd.ListObjectsInfo, err error) {
	return cmd.ListObjectsInfo{
		IsTruncated: false,
		Objects: []cmd.ObjectInfo{{
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
	data *hash.Reader, metadata map[string]string) (objInfo cmd.ObjectInfo,
	err error) {
	panic("TODO")
}

func (s *storjObjects) Shutdown(context.Context) error {
	panic("TODO")
}

func (s *storjObjects) StorageInfo(context.Context) cmd.StorageInfo {
	return cmd.StorageInfo{}
}
