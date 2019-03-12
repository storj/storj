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
	upload *Upload
}

func NewBufferedUpload(upload *Upload, size int) Uploader {
	buffered := &BufferedUpload{}
	buffered.upload = upload
	buffered.buffer = *bufio.NewWriterSize(buffered.upload, size)
	return buffered
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
	download *Download
}

func NewBufferedDownload(download *Download, size int) Downloader {
	buffered := &BufferedDownload{}
	buffered.download = download
	buffered.buffer = *bufio.NewReaderSize(buffered.download, size)
	return buffered
}

func (download *BufferedDownload) Read(p []byte) (int, error) {
	return download.buffer.Read(p)
}

func (download *BufferedDownload) Close() error {
	return download.download.Close()
}
