package downloader

import (
	minio "github.com/minio/minio/cmd"
	"fmt"
)

type ObjectDownloadError struct {
	Bi minio.ObjectInfo
	Err error
}

func NewObjectDownloadError(bi minio.ObjectInfo, err error) *ObjectDownloadError {
	return &ObjectDownloadError{bi, err}
}

func (objDwnErr *ObjectDownloadError) Error() string {
	return fmt.Sprintf("Download error: b %s, n %s. Error: %s", objDwnErr.Bi.Bucket, objDwnErr.Bi.Name, objDwnErr.Err)
}
