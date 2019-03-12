// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bufio"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

type BufferedUpload struct {
	buffer bufio.Writer
	upload Upload
}

func (upload *BufferedUpload) Init() {
	upload.buffer.Reset(&upload.upload)
}

func (upload *BufferedUpload) Write(data []byte) (int, error) {
	return upload.buffer.Write(data)
}

func (upload *BufferedUpload) Close() (*pb.PieceHash, error) {
	flushErr := upload.buffer.Flush()
	piece, closeErr := upload.upload.Close()
	return piece, errs.Combine(flushErr, closeErr)
}

type BufferedDownload struct {
	buffer   bufio.Reader
	download Download
}

func (download *BufferedDownload) Init() {
	download.buffer.Reset(&download.download)
}

func (download *BufferedDownload) Read(p []byte) (int, error) {
	return download.buffer.Read(p)
}

func (download *BufferedDownload) Close() error {
	return download.download.Close()
}
