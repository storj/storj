package put

import (
	"path"
	"storj.io/mirroring/utils"

	minio "github.com/minio/minio/cmd"
	"fmt"
)

type AsyncUploader interface {
	UploadFileAsync(ctx PutContext, bucket, lpath string) <-chan error
	UploadFolderAsync(ctx PutContext, bucket, lpath string) <-chan error
}

type ObjLayerAsyncUploader interface {
	AsyncUploader
	SetObjLayer(minio.ObjectLayer)
}

type folderUploader struct {
	fileUploader
	DirReader
	utils.Logger
}

func NewFolderUploader(ol minio.ObjectLayer, hr *hFileReader, dr DirReader, lg utils.Logger) *folderUploader {
	return &folderUploader{fileUploader{objectUploader{ol}, hr}, dr, lg}
}

func (f *folderUploader) SetObjLayer(layer minio.ObjectLayer) {
	if f.ol != nil {
		return
	}

	f.ol = layer
}

func (f *folderUploader) uploadDir(ctx PutContext, bucket string, dir Dir) {
	files := dir.Files()
	for i := 0; i < len(files); i++ {
		item := files[i]

		res := <-f.fileUploader.UploadFileAsync(ctx, bucket, path.Join(dir.Path(), item.Name()))
		f.LogE(res.err)
		if res.err == nil {
			f.Log(fmt.Sprintf("Successfully uploaded %s", res.oi.Name))
		}
	}

	if !ctx.Recursive(){
		return
	}

	dirs := dir.Dirs()
	for i := 0; i < len(dirs); i++ {
		item := dirs[i]

		ctxf := ctx.WithPrefix(path.Join(ctx.Prefix(), item.Name()))
		f.Log(fmt.Sprintf("Recursively uploading folder %s", ctxf.Prefix()))
		_ = <-f.UploadFolderAsync(ctxf, bucket, path.Join(dir.Path(), item.Name()))
	}
}

func (f *folderUploader) UploadFileAsync(ctx PutContext, bucket, lpath string) <-chan error {
	derrc := make(chan error, 1)
	resc := f.fileUploader.UploadFileAsync(ctx, bucket, lpath)

	go func() {
		res := <-resc
		if res.err == nil {
			f.Log(fmt.Sprintf("Successfully uploaded %s", res.oi.Name))
		}
		derrc <- res.err
	}()

	return derrc
}

func (f *folderUploader) UploadFolderAsync(ctx PutContext, bucket, lpath string) <-chan error {
	derrc := make(chan error, 1)

	dir, err := f.ReadDir(lpath)
	if err != nil {
		derrc <- err
		return derrc
	}

	//upload
	go func() {
		defer func() {
			derrc <- nil
		}()

		f.uploadDir(ctx, bucket, dir)
	}()

	return derrc
}
