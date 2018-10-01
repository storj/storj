package put

import (
	"github.com/minio/minio/pkg/hash"
	"context"
	minio "github.com/minio/minio/cmd"
)

type uploadResult struct {
	oi minio.ObjectInfo
	err error
}

type objectUploader struct {
	ol minio.ObjectLayer
}

func (u *objectUploader) UploadObjectAsync(ctx context.Context, bucket, object string, data *hash.Reader) <-chan uploadResult {
	resc := make(chan uploadResult)

	utask := func(resc chan<- uploadResult) {
		oi, err := u.ol.PutObject(ctx, bucket, object, data, make(map[string]string), minio.ObjectOptions{})
		
		res := uploadResult{}
		res.oi = oi
		res.err = err
		resc<-res
	}

	go utask(resc)
	return resc
}