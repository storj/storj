package put

import (
	"storj.io/mirroring/cmd/utils"
	"context"
	"errors"
	minio "github.com/minio/minio/cmd"
)

func checkObj(ctx context.Context, ol minio.ObjectLayer, bucket, object string) error {
	//TODO: check opts
	_, err := ol.GetObjectInfo(ctx, bucket, object, minio.ObjectOptions{})
	if err == nil {
		return errors.New("object allready exists") //Make or find approp error
	}

	return nil
}

type fileUploader struct {
	objectUploader
	*hFileReader
}

func (u *fileUploader) UploadFileAsync(ctx PutContext, bucket, lpath string) <-chan uploadResult {
	dresc := make(chan uploadResult, 1) // delayed result chanel for error handling
	res := uploadResult{}

	hfreader, err := u.ReadFileH(lpath)
	if err != nil {
		res.err = err
		dresc <- res
		return dresc
	}

	object := utils.GetObjectName(hfreader.FileInfo().Name(), ctx.Prefix(), ctx.Delimiter())

	if !ctx.Force() {
		err = checkObj(ctx, u.ol, bucket, object)
		if err != nil {
			res.err = err
			dresc <- res
			return dresc
		}
	}

	return u.UploadObjectAsync(ctx, bucket, object, hfreader.HashReader())
}
