package downloader

import (
	"fmt"
	"os"
	minio "github.com/minio/minio/cmd"
	"io"
	"context"
	"path"
	)

func NewDownloader(bck minio.ObjectLayer, cwd string, prms *Params) *downloader {
	if prms == nil {
		prms = NewDefaultParams()
	}

	return &downloader{bck, os.Stdout, prms, cwd}
}

type downloader struct {
	bck minio.ObjectLayer
	logger io.ReadWriter//utils.Logger
	prms *Params
	cwd string
}

func (d *downloader) Params() Params {
	return *d.prms
}

func (d *downloader) SetParams(prms Params) {
	if prms.delimiter == "" {
		prms.delimiter = DEFAULT_DELIMITER
	}

	d.prms = &prms
}

func (d *downloader) GetBucket(bucketName string) (err error) {
	loi, err := d.listObjects(nil, *d.prms, bucketName)
	if err != nil {
		d.logf("Failed to get files list from bucket %s\n", bucketName)
		return
	}

	prms := *d.prms
	if prms.path == "" {
		prms.path = path.Join(bucketName, prms.prefix)
		prms.rootPath = path.Join(bucketName)
	}

	//Need to handle partial prefix!....
	if d.prms.recursive {
		d.bucketDownloadRecursive(prms, loi) //get error?
	} else {
		d.bucketDownload(prms, loi)
	}

	return nil
}

func (d *downloader) bucketDownload(prms Params, loi ListObjInfo) {
	if len(loi.Objects) == 0 {
		return
	}

	//Should be separate func?
	if !FileExists(prms.path) && os.MkdirAll(prms.path, os.ModePerm) != nil {
		return // Error?
	}

	d.bucketDownloadObjects(prms, loi)
}

func (d *downloader) bucketDownloadObjects(prms Params, loi ListObjInfo) {
	length := len(loi.Objects)
	errCh := make(chan *ObjectDownloadError)
	var count int
	var errCount int

	for i := range loi.Objects {
		obj := loi.Objects[i]
		//We can bu sure that obj.Name can not be empty string
		objName := StripPrefix(obj.Name)
		//filePath := ResolveFipalePathForFolder(prms.path, d.cwd, objName) //Bug?
		filePath := path.Join(prms.path, objName)

		go d.downloadObject(filePath, &obj, errCh)
	}

	for er := range errCh {
		if er.Err == nil {
			d.logf("Successfullv downloaded %s/%s\n", er.Bi.Bucket, er.Bi.Name)
		} else {
			errCount++
			d.log(er.Error() + "\n")
		}

		count++
		if count == length {
			close(errCh)
		}
	}
}

func splitFirstPrefix(firstPrefix, prefix string) string {
	length := len(firstPrefix)
	if firstPrefix != "" && firstPrefix == prefix[:length] {
		return prefix[length:]
	}

	return prefix
}

func (d *downloader) bucketDownloadRecursive(prms Params, loi ListObjInfo) {
	d.logf("Downloading objects from %s/%s\n", loi.Bucket, prms.prefix)

	if len(loi.Objects) != 0 {
		if !FileExists(prms.path) && os.MkdirAll(prms.path, os.ModePerm) != nil {
			return // Error?
		}

		d.bucketDownloadObjects(prms, loi)
	}

	for i := range loi.Prefixes {
		prfx := loi.Prefixes[i]

		//copy params
		prmsp := prms
		prmsp.prefix = prfx
		prmsp.path = path.Join(prmsp.rootPath, prfx)

		loip, err := d.listObjects(nil, prmsp, loi.Bucket)
		if err != nil {
			continue
		}

		d.bucketDownloadRecursive(prmsp, loip)
	}
}

func (d *downloader) GetObject(bucket, objectName string) (err error) {
	info, err := d.bck.GetObjectInfo(nil, bucket, objectName, minio.ObjectOptions{})
	if err != nil {
		return
	}

	//should add prefix and bucket if no path specified?? Noo
	filePath := ResolveFilePathForObject(d.prms.path, d.cwd, objectName)
	dir, _ := path.Split(filePath)
	if !FileExists(dir) && os.MkdirAll(dir, os.ModePerm) != nil {
		return // Error?
	}

	err = d.downloadObject(filePath, &info, nil)
	return
}

func (d *downloader) downloadObject(filePath string, bi *minio.ObjectInfo, errCh chan<-*ObjectDownloadError) (err error) {
	defer func() {
		if errCh != nil {
			errCh<-NewObjectDownloadError(*bi, err)
		}
	}()

	if FileExists(filePath) {
		return fmt.Errorf("There is a file allready with given filename %s\n", filePath)
	}

	w, err := os.Create(filePath)
	if err != nil {
		return
	}

	errChInner := make(chan error)

	go minioDownloadTask(d.bck, bi.Bucket, bi.Name, bi.ETag, bi.Size, w, errChInner)
	if err = <-errChInner; err != nil {
		d.removeFile(filePath)
		return
	}

	return
}

func minioDownloadTask(mirr minio.ObjectLayer, bucket, object, etag string, size int64, w io.WriteCloser, errCh chan<-error) (err error) {
	defer w.Close()
	defer func () {
		if errCh != nil {
			errCh<-err
			close(errCh)
		}
	}()

	err = mirr.GetObject(nil, bucket, object, 0, size, w, etag, minio.ObjectOptions{})
	if err != nil {
		return
	}

	return
}

func (d *downloader) listObjectsV1(ctx context.Context, prms Params, bucketName string) (lsObjInfo ListObjInfo, err error) {
	loiV2, err := d.bck.ListObjectsV2(
		ctx, bucketName,
		prms.prefix,
		prms.token,
		prms.delimiter,
		prms.maxKeys,
		prms.fetchOwner,
		prms.startAfter)

	if err != nil {
		return
	}

	lsObjInfo.Bucket = bucketName
	lsObjInfo.Objects = loiV2.Objects
	lsObjInfo.Prefixes = loiV2.Prefixes
	return
}

func (d *downloader) listObjectsV2(ctx context.Context, prms Params, bucketName string) (lsObjInfo ListObjInfo, err error) {
	loiV1, err := d.bck.ListObjects(
		ctx, bucketName,
		prms.prefix,
		prms.marker,
		prms.delimiter,
		prms.maxKeys)

	if err != nil {
		return
	}

	lsObjInfo.Bucket = bucketName
	lsObjInfo.Objects = loiV1.Objects
	lsObjInfo.Prefixes = loiV1.Prefixes
	return
}

func (d *downloader) listObjects(ctx context.Context, prms Params, bucketName string) (ListObjInfo, error) {
	lsObjInfo, err := d.listObjectsV2(ctx, prms, bucketName)
	if err == nil {
		lsObjInfo, err = d.listObjectsV1(ctx, prms, bucketName)
	}

	return lsObjInfo, err
}

func (d *downloader) removeFile(filePath string) (err error) {
	err = os.Remove(filePath)
	if err != nil {
		d.logf("Unable to delete file %s. Error: %s\n", filePath, err)
	}

	return
}

func (d *downloader) log(message string) {
	d.logger.Write([]byte(message))
}

func (d *downloader) logf(fstring string, args ...interface{}) {
	d.log(fmt.Sprintf(fstring, args...))
}

type ListObjInfo struct {
	Bucket string
	Objects []minio.ObjectInfo
	Prefixes []string
}

