package put

import (
	"testing"
	tutils "storj.io/mirroring/utils/testing_utils"
	minio "github.com/minio/minio/cmd"
	"context"
	"github.com/minio/minio/pkg/hash"
	"errors"
	"time"
)

func TestExec(t *testing.T) {
	testError := errors.New("Test error")

	putError := func(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (minio.ObjectInfo, error) {
		return minio.ObjectInfo{}, testError
	}

	_ = func(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (minio.ObjectInfo, error) {
		time.Sleep(time.Second * 1)

		objInfo := minio.ObjectInfo{
			Bucket: bucket,
			Name: object,
		}

		return objInfo, nil
	}

	cases := []struct {
		testName string
		testFunc func(t *testing.T)
	} {
		{
			"",
			func(t *testing.T) {
				mirr := tutils.NewProxyObjectLayer()
				mirr.PutObjectFunc = putError

				gw := &tutils.MockGateway{mirr}
				lg := &tutils.MockLogger{}

				uploader := NewFolderUploader(mirr, NewHFileReader(), &dirReader{}, lg)
				exec := newPutExec(gw, uploader, lg)
				_ = exec.runE(nil, []string{"bucket", "localpath"})
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, c.testFunc)
	}
}
