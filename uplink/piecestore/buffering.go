// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bufio"
	"sync"

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

type LockingUpload struct {
	mu     sync.Mutex
	upload Uploader
}

func (upload *LockingUpload) Write(p []byte) (int, error) {
	upload.mu.Lock()
	defer upload.mu.Unlock()
	return upload.upload.Write(p)
}

func (upload *LockingUpload) Close() (*pb.PieceHash, error) {
	upload.mu.Lock()
	defer upload.mu.Unlock()
	return upload.upload.Close()
}

type LockingDownload struct {
	mu       sync.Mutex
	download Downloader
}

func (download *LockingDownload) Read(p []byte) (int, error) {
	download.mu.Lock()
	defer download.mu.Unlock()
	return download.download.Read(p)
}

func (download *LockingDownload) Close() error {
	download.mu.Lock()
	defer download.mu.Unlock()
	return download.download.Close()
}
