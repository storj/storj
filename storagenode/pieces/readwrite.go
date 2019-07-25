// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"hash"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/storage"
)

const (
	// V1PieceHeaderSize is the size of the piece header used by piece storage format 1 (.sj1).
	// It has a constant size because:
	//
	//  * we do not anticipate needing more than this
	//  * we will be able to sum up all space used by a satellite (or all satellites) without
	//    opening and reading from each piece file
	//  * this simplifies piece file writing (no need to precalculate the necessary header
	//    size before writing)
	//
	// If more space than this is needed, we will need to use a new storage format version.
	V1PieceHeaderSize = 128
)

// Writer implements a piece writer that writes content to blob store and calculates a hash.
type Writer struct {
	hash     hash.Hash
	blob     storage.BlobWriter
	dataSize int64 // piece size only; i.e., not including piece header

	closed bool
}

// NewWriter creates a new writer for storage.BlobWriter.
func NewWriter(blob storage.BlobWriter, formatVersion storage.FormatVersion) (*Writer, error) {
	w := &Writer{}
	if formatVersion < storage.FormatV1 {
		return nil, Error.New("writing to storage format version 0 is not supported")
	}
	// skip header area for now; fill in on commit
	if _, err := blob.Seek(V1PieceHeaderSize, io.SeekStart); err != nil {
		return nil, Error.Wrap(err)
	}
	w.blob = blob
	w.hash = pkcrypto.NewHash()
	return w, nil
}

// Write writes data to the blob and calculates the hash.
func (w *Writer) Write(data []byte) (int, error) {
	n, err := w.blob.Write(data)
	w.dataSize += int64(n)
	_, _ = w.hash.Write(data[:n]) // guaranteed not to return an error
	if err == io.EOF {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Size returns the amount of data written to the piece so far, not including the size of
// the piece header.
func (w *Writer) Size() int64 { return w.dataSize }

// Hash returns the hash of data written so far.
func (w *Writer) Hash() []byte { return w.hash.Sum(nil) }

// Commit commits piece to permanent storage.
func (w *Writer) Commit(ctx context.Context, pieceHeader *pb.PieceHeader) (err error) {
	defer mon.Task()(&ctx)(&err)
	if w.closed {
		return Error.New("already closed")
	}

	// point of no return: after this we definitely either commit or cancel
	w.closed = true
	defer func() {
		if err != nil {
			err = Error.Wrap(errs.Combine(err, w.blob.Cancel(ctx)))
		} else {
			err = Error.Wrap(w.blob.Commit(ctx))
		}
	}()

	pieceHeader.FormatVersion = int32(w.blob.GetStorageFormatVersion())
	headerBytes, err := proto.Marshal(pieceHeader)
	if err != nil {
		return err
	}
	if len(headerBytes) > V1PieceHeaderSize {
		// This should never happen under normal circumstances, and it might deserve a panic(),
		// but I'm not *entirely* sure this case can't be triggered by a malicious uplink. Are
		// google.protobuf.Timestamp fields variable-width?
		return Error.New("marshaled piece header too big!")
	}
	if _, err := w.blob.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err = w.blob.Write(headerBytes); err != nil {
		return Error.New("failed writing piece header at file start: %v", err)
	}
	// seek back to the end, as blob.Commit will truncate from the current file position
	if _, err := w.blob.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	return nil
}

// Cancel deletes any temporarily written data.
func (w *Writer) Cancel(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if w.closed {
		return nil
	}
	w.closed = true
	return Error.Wrap(w.blob.Cancel(ctx))
}

// Reader implements a piece reader that reads content from blob store.
type Reader struct {
	formatVersion storage.FormatVersion

	blob storage.BlobReader
	pos  int64 // relative to file start; i.e., it includes piece header
	size int64 // piece size only; i.e., not including piece header
}

// NewReader creates a new reader for storage.BlobReader.
func NewReader(blob storage.BlobReader) (*Reader, error) {
	size, err := blob.Size()
	if err != nil {
		return nil, Error.Wrap(err)
	}
	formatVersion := blob.GetStorageFormatVersion()
	if formatVersion >= storage.FormatV1 && size < V1PieceHeaderSize {
		return nil, Error.New("invalid piece file for storage format version %d: too small for header (%d < %d)", formatVersion, size, V1PieceHeaderSize)
	}

	reader := &Reader{
		formatVersion: formatVersion,
		blob:          blob,
		size:          size - V1PieceHeaderSize,
	}
	return reader, nil
}

// GetPieceHeader reads, unmarshals, and returns the piece header. It may only be called
// before any Read() calls. (Retrieving the header at any time could be supported, but for
// the sake of performance we need to understand why and how often that would happen.)
func (r *Reader) GetPieceHeader() (*pb.PieceHeader, error) {
	if r.formatVersion < storage.FormatV1 {
		return nil, Error.New("Can't get piece header from storage format V0 reader")
	}
	if r.pos != 0 {
		return nil, Error.New("GetPieceHeader called when not at the beginning of the blob stream")
	}
	var headerBytes [V1PieceHeaderSize]byte
	n, err := r.blob.Read(headerBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	r.pos += int64(n)
	header := &pb.PieceHeader{}
	if err := proto.Unmarshal(headerBytes[:], header); err != nil {
		return nil, Error.New("piece header: %v", err)
	}
	return header, nil
}

// Read reads data from the underlying blob, buffering as necessary.
func (r *Reader) Read(data []byte) (int, error) {
	if r.formatVersion >= storage.FormatV1 && r.pos < V1PieceHeaderSize {
		// should only be necessary once per reader. or zero times, if GetPieceHeader is used
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return 0, Error.Wrap(err)
		}
	}
	n, err := r.blob.Read(data)
	r.pos += int64(n)
	if err == io.EOF {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Seek seeks to the specified location within the piece content (ignoring the header).
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekStart && r.formatVersion >= storage.FormatV1 {
		offset += V1PieceHeaderSize
	}
	if whence == io.SeekStart && r.pos == offset {
		return r.pos, nil
	}

	pos, err := r.blob.Seek(offset, whence)
	r.pos = pos
	if r.formatVersion >= storage.FormatV1 {
		pos -= V1PieceHeaderSize
	}
	if err == io.EOF {
		return pos, err
	}
	return pos, Error.Wrap(err)
}

// ReadAt reads data at the specified offset
func (r *Reader) ReadAt(data []byte, offset int64) (int, error) {
	if r.formatVersion >= storage.FormatV1 {
		offset += V1PieceHeaderSize
	}
	n, err := r.blob.ReadAt(data, offset)
	if err == io.EOF {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Size returns the amount of data in the piece.
func (r *Reader) Size() int64 { return r.size }

// Close closes the reader.
func (r *Reader) Close() error {
	return Error.Wrap(r.blob.Close())
}
