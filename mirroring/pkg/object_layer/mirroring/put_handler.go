package mirroring

import (
	"io"
	"github.com/minio/minio/pkg/hash"
	"context"
	"storj.io/mirroring/utils"
	minio "github.com/minio/minio/cmd"
)

type asyncHandler struct {
	ol minio.ObjectLayer
}

func (h *asyncHandler) putAsync(ctx context.Context, oi *minio.ObjectInfo, bucket, object string, metadata map[string]string, data *hash.Reader, opts minio.ObjectOptions) (<-chan error) {
	errc := make(chan error)
	putTask := func(errc chan<- error) {
		_oi, err := h.ol.PutObject(ctx, bucket, object, data, metadata, opts)
		_oi.Name = object
		*oi = _oi

		errc<-err
		//close(errc) Cant close it nor make it nil
	}

	go putTask(errc)
	return errc
}

type putHandler struct {
	main, mirr asyncHandler
	logger utils.Logger
}

func newPutHandler(main, mirr minio.ObjectLayer, lg utils.Logger) *putHandler {
	return &putHandler{asyncHandler{main}, asyncHandler{mirr}, lg}
}

func (h *putHandler) process(ctx context.Context, bucket, object string, data *hash.Reader, metadata map[string]string, opts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
	pr, pw := io.Pipe()
	teer := io.TeeReader(data, pw)

	rmain, err := hash.NewReader(teer, data.Size(), data.MD5HexString(), data.SHA256HexString())
	if err != nil {
		return
	}

	rmirr, err := hash.NewReader(pr, data.Size(), data.MD5HexString(), data.SHA256HexString())
	if err != nil {
		return
	}

	ctxm, mcancelf := context.WithCancel(ctx)
	ctxmr, mrcancelf := context.WithCancel(ctx)
	defer func() {
		mcancelf()
		mrcancelf()
	}()

	var moi, mroi minio.ObjectInfo
	errMain := h.main.putAsync(ctxm, &moi, bucket, object, metadata, rmain, opts)
	errMirr := h.mirr.putAsync(ctxmr, &mroi, bucket, object, metadata, rmirr, opts)

	tnum := 2 //if both put operations done we won't handle cancelation
	done := ctx.Done()
	for i := 0; i < tnum; i++ {
		select {
		case err = <-errMain:
			h.logger.LogE(err)
			objInfo = moi
			if err != nil {
				pr.Close()
				mrcancelf() //Not sure if we need to call it cause it autocanceled once pipe writer s closed
			}
		case errm := <-errMirr:
			h.logger.LogE(errm)
		case <-done:
			mcancelf()
			pr.Close()
			tnum++ //Ensure that we wont try to close pipe again? Will panic? Try to catch all errors.
			done = nil // dont want to track closed chanel
		}
	}

	return
}