package put

import (
	"storj.io/mirroring/utils"
	futils "storj.io/mirroring/cmd/utils"
	"fmt"
	"path"
	"os"
	"context"
	"github.com/minio/minio/pkg/auth"
	"github.com/spf13/cobra"
	minio "github.com/minio/minio/cmd"
)

type putExec struct {
	gw minio.Gateway
	logger utils.Logger
	ObjLayerAsyncUploader
}

func NewPutExec(gw minio.Gateway, logger utils.Logger) putExec {
	uploader := NewFolderUploader(nil, NewHFileReader(), &dirReader{}, logger)
	return newPutExec(gw, uploader, logger)
}

func newPutExec(gw minio.Gateway, uploader ObjLayerAsyncUploader, logger utils.Logger) putExec {
	return putExec{gw, logger, uploader }
}

func (e putExec) logF(format string, params ...interface{}) {
	e.logger.Log(fmt.Sprintf(format, params))
}

//Main function
func (e putExec) runE(cmd *cobra.Command, args []string) error {
	mirr, err := e.gw.NewGatewayLayer(auth.Credentials{})
	if err != nil {
		return err
	}

	e.SetObjLayer(mirr)

	bctx := context.Background()
	_, err = mirr.GetBucketInfo(bctx, args[0])
	if err != nil {
		return err
	}

	isDir, err := futils.CheckIfDir(args[1])
	if err != nil {
		return err
	}

	ctx, cancelf := context.WithCancel(bctx)
	defer func() {
		cancelf()
	}()
	
	cwd, _ := os.Getwd()
	lpath := path.Join(cwd, args[1])

	ctxp := NewPutCtx(
		ctx,
		frecursive,
		fforce,
		fprefix,
		fdelimiter)

	var errc <-chan error
	if isDir {
		errc = e.UploadFolderAsync(ctxp, args[0], lpath)
	} else {
		errc = e.UploadFileAsync(ctxp, args[0], lpath)
	}

	tnum := 1
	for i:= 0; i < tnum; i++ {
		select {
		case err = <-errc:
			e.logger.LogE(err)
		case sig := <-sigc:
			e.logF("Catched interrupt! %s\n", sig)
			cancelf()
			tnum++
		}
	}

	return err
}