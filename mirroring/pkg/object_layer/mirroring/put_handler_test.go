package mirroring

import (
	tutils "storj.io/mirroring/utils/testing_utils"
	"testing"
	"github.com/minio/minio/cmd"
	"context"
	"github.com/minio/minio/pkg/hash"
	minio "github.com/minio/minio/cmd"
	"github.com/stretchr/testify/assert"
	"bytes"
	"errors"
	"time"
)

type putFunc func(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts cmd.ObjectOptions) (cmd.ObjectInfo, error)

func getPutMockFunc(oi *minio.ObjectInfo, err error) putFunc {
	return func(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (minio.ObjectInfo, error) {
		time.Sleep(time.Second) //Mock work

		if oi == nil {
			return minio.ObjectInfo{}, err
		}

		return *oi, err
	}
}

func TestPutHandler(t *testing.T) {
	prime := tutils.NewProxyObjectLayer()
	alter := tutils.NewProxyObjectLayer()

	m := MirroringObjectLayer{
		Prime: prime,
		Alter: alter,
		Logger: nil,
	}

	testError := errors.New("test error")

	putNoErr := getPutMockFunc(nil, nil)
	putErr := getPutMockFunc(nil, testError)

	ctxb := context.Background()
	buff := []byte("test")

	cases := []struct {
		testName, address string
		testFunc          func(*testing.T)
	}{
		{
			testName: "No Error",
			testFunc: func (*testing.T) {
				lg := &tutils.MockLogger{}
				m.Logger = lg

				prime.PutObjectFunc = putNoErr
				alter.PutObjectFunc = putNoErr

				data, err := hash.NewReader(bytes.NewReader(buff), int64(len(buff)), "", "")
				assert.NoError(t, err)

				_, err = m.PutObject(ctxb, "bucket", "object", data, nil, minio.ObjectOptions{})
				assert.NoError(t, err)
				assert.Equal(t, 2, lg.LogECount())

				prm, err := lg.GetLastLogEParam()
				assert.NoError(t, err)
				assert.Equal(t, nil, prm)
			},

		},
		{
			testName: "Err main",
			testFunc: func (*testing.T) {
				lg := &tutils.MockLogger{}
				m.Logger = lg

				prime.PutObjectFunc = putErr
				alter.PutObjectFunc = putNoErr

				data, err := hash.NewReader(bytes.NewReader(buff), int64(len(buff)), "", "")
				assert.NoError(t, err)

				_, err = m.PutObject(ctxb, "bucket", "object", data, nil, minio.ObjectOptions{})
				assert.Equal(t, testError, err)
				assert.Equal(t, 2, lg.LogECount())
			},
		},
		{
			testName: "Context cancel",
			testFunc: func (*testing.T) {
				lg := &tutils.MockLogger{}
				m.Logger = lg

				prime.PutObjectFunc = putNoErr
				alter.PutObjectFunc = putNoErr

				data, err := hash.NewReader(bytes.NewReader(buff), int64(len(buff)), "", "")
				assert.NoError(t, err)

				ctxc, cancelf := context.WithCancel(ctxb)
				cancelf()

				_, err = m.PutObject(ctxc, "bucket", "object", data, nil, minio.ObjectOptions{})
				assert.NoError(t, err)
				assert.Equal(t, 2, lg.LogECount())

				prm, err := lg.GetLastLogEParam()
				assert.NoError(t, err)
				assert.Equal(t, nil, prm)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, c.testFunc)
	}
}
