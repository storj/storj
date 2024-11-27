// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"encoding/binary"
	"errors"
	"hash"
	"io"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
)

const (
	// V1PieceHeaderReservedArea is the amount of space to be reserved at the beginning of
	// pieces stored with filestore.FormatV1 or greater. Serialized piece headers should be
	// written into that space, and the remaining space afterward should be zeroes.
	// V1PieceHeaderReservedArea includes the size of the framing field
	// (v1PieceHeaderFrameSize). It has a constant size because:
	//
	//  * We do not anticipate needing more than this.
	//  * We will be able to sum up all space used by a satellite (or all satellites) without
	//    opening and reading from each piece file (stat() is faster than open()).
	//  * This simplifies piece file writing (if we needed to know the exact header size
	//    before writing, then we'd need to spool the entire contents of the piece somewhere
	//    before we could calculate the hash and size). This way, we can simply reserve the
	//    header space, write the piece content as it comes in, and then seek back to the
	//    beginning and fill in the header.
	//
	// We put it at the beginning of piece files because:
	//
	//  * If we put it at the end instead, we would have to seek to the end of a file (to find
	//    out the real size while avoiding race conditions with stat()) and then seek backward
	//    again to get the header, and then seek back to the beginning to get the content.
	//    Seeking on spinning platter hard drives is very slow compared to reading sequential
	//    bytes.
	//  * Putting the header in the middle of piece files might be entertaining, but it would
	//    also be silly.
	//  * If piece files are incorrectly truncated or not completely written, it will be
	//    much easier to identify those cases when the header is intact and findable.
	//
	// If more space than this is needed, we will need to use a new storage format version.
	V1PieceHeaderReservedArea = 512

	// v1PieceHeaderFramingSize is the size of the field used at the beginning of piece
	// files to indicate the size of the marshaled piece header within the reserved header
	// area (because protobufs are not self-delimiting, which is lame).
	v1PieceHeaderFramingSize = 2
)

// BadFormatVersion is returned when a storage format cannot support the request function.
var BadFormatVersion = errs.Class("Incompatible storage format version")

// Writer implements a piece writer that writes content to blob store and calculates a hash.
type Writer struct {
	log       *zap.Logger
	hash      hash.Hash
	blob      blobstore.BlobWriter
	pieceSize int64 // piece size only; i.e., not including piece header

	blobs     blobstore.Blobs
	satellite storj.NodeID
	closed    bool
}

// NewWriter creates a new writer for blobstore.BlobWriter.
func NewWriter(log *zap.Logger, blobWriter blobstore.BlobWriter, blobs blobstore.Blobs, satellite storj.NodeID, hashAlgorithm pb.PieceHashAlgorithm) (*Writer, error) {
	w := &Writer{log: log}
	if blobWriter.StorageFormatVersion() >= filestore.FormatV1 {
		// We are reserving header area for now- we want the header to be at the
		// beginning of the file, to make it quick to seek there and also to make it easier
		// to identify situations where a blob file has been truncated incorrectly. And we
		// don't know what exactly is going to be in the header yet--we won't know what the
		// hash or size or timestamp or expiration or signature fields need to be until we
		// have received the whole piece.
		//
		// Once the writer calls Commit() on this writer, we will seek back to the beginning
		// of the file and write the header.
		if err := blobWriter.ReserveHeader(int64(V1PieceHeaderReservedArea)); err != nil {
			return nil, Error.Wrap(err)
		}
	}
	w.blob = MonitorBlobWriter("pieces_writer_io", blobWriter)

	w.hash = MonitorHash("pieces_writer_hash", pb.NewHashFromAlgorithm(hashAlgorithm))

	w.blobs = blobs
	w.satellite = satellite
	return w, nil
}

// Write writes data to the blob and calculates the hash.
func (w *Writer) Write(data []byte) (int, error) {
	n, err := w.blob.Write(data)
	w.pieceSize += int64(n)
	_, _ = w.hash.Write(data[:n]) // guaranteed not to return an error
	if errors.Is(err, io.EOF) {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Size returns the amount of data written to the piece so far, not including the size of
// the piece header.
func (w *Writer) Size() int64 { return w.pieceSize }

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

	// if the blob store is a cache, update the cache, but only if we did not
	// encounter an error
	if cache, ok := w.blobs.(*BlobsUsageCache); ok {
		defer func() {
			if err == nil {
				totalSize, sizeErr := w.blob.Size()
				if sizeErr != nil {
					w.log.Error("Failed to calculate piece size, cannot update the cache",
						zap.Error(sizeErr), zap.Stringer("piece ID", pieceHeader.GetOrderLimit().PieceId),
						zap.Stringer("satellite ID", w.satellite))
					return
				}
				cache.Update(ctx, w.satellite, totalSize, w.Size(), 0)
			}
		}()
	}

	formatVer := w.blob.StorageFormatVersion()
	if formatVer == filestore.FormatV0 {
		return nil
	}

	pieceHeader.FormatVersion = pb.PieceHeader_FormatVersion(formatVer)
	headerBytes, err := pb.Marshal(pieceHeader)
	if err != nil {
		return Error.Wrap(err)
	}

	mon.IntVal("storagenode_pieces_pieceheader_size").Observe(int64(len(headerBytes)))
	if len(headerBytes) > (V1PieceHeaderReservedArea - v1PieceHeaderFramingSize) {
		// This should never happen under normal circumstances, and it might deserve a panic(),
		// but I'm not *entirely* sure this case can't be triggered by a malicious uplink. Are
		// google.protobuf.Timestamp fields variable-width?
		mon.Meter("storagenode_pieces_pieceheader_overflow").Mark(len(headerBytes))
		return Error.New("marshaled piece header too big!")
	}

	// keep track of the size now so that we can seek back to it later (see below).
	size, err := w.blob.Size()
	if err != nil {
		return Error.Wrap(err)
	}
	if _, err := w.blob.Seek(0, io.SeekStart); err != nil {
		return Error.Wrap(err)
	}

	// We need to store some "framing" bytes first, because protobufs are not self-delimiting.
	// In cases where the serialized pieceHeader is not exactly V1PieceHeaderReservedArea bytes
	// (probably _all_ cases), without this marker, we wouldn't have any way to take the
	// V1PieceHeaderReservedArea bytes from a piece blob and trim off the right number of zeroes
	// at the end so that the protobuf unmarshals correctly.
	var fullHeader [V1PieceHeaderReservedArea]byte
	binary.BigEndian.PutUint16(fullHeader[0:2], uint16(len(headerBytes)))
	copy(fullHeader[2:], headerBytes)

	if _, err = w.blob.Write(fullHeader[:]); err != nil {
		return Error.New("failed writing piece header at file start: %w", err)
	}

	// seek back to the end, as blob.Commit will truncate from the current file position.
	// (don't try to seek(0, io.SeekEnd), because dir.CreateTemporaryFile preallocs space
	// and the actual end of the file might be far past the intended end of the piece.)
	if _, err := w.blob.Seek(size, io.SeekStart); err != nil {
		return Error.Wrap(err)
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
	formatVersion blobstore.FormatVersion

	blob      blobstore.BlobReader
	pos       int64 // relative to file start; i.e., it includes piece header
	pieceSize int64 // piece size only; i.e., not including piece header
}

// NewReader creates a new reader for blobstore.BlobReader.
func NewReader(blob blobstore.BlobReader) (*Reader, error) {
	size, err := blob.Size()
	if err != nil {
		return nil, Error.Wrap(err)
	}
	formatVersion := blob.StorageFormatVersion()
	if formatVersion >= filestore.FormatV1 {
		if size < V1PieceHeaderReservedArea {
			return nil, Error.New("invalid piece file for storage format version %d: too small for header (%d < %d)", formatVersion, size, V1PieceHeaderReservedArea)
		}
		size -= V1PieceHeaderReservedArea
	}

	reader := &Reader{
		formatVersion: formatVersion,
		blob:          blob,
		pieceSize:     size,
	}
	return reader, nil
}

// StorageFormatVersion returns the storage format version of the piece being read.
func (r *Reader) StorageFormatVersion() blobstore.FormatVersion {
	return r.formatVersion
}

// GetPieceHeader reads, unmarshals, and returns the piece header. It may only be called once,
// before any Read() calls.
//
// Retrieving the header at any time could be supported, but for the sake
// of performance we need to understand why and how often that would happen.
func (r *Reader) GetPieceHeader() (*pb.PieceHeader, error) {
	if r.formatVersion < filestore.FormatV1 {
		return nil, BadFormatVersion.New("Can't get piece header from storage format V0 reader")
	}
	if r.pos != 0 {
		return nil, Error.New("GetPieceHeader called when not at the beginning of the blob stream")
	}
	// We need to read the size of the serialized header protobuf before we read the header
	// itself. The headers aren't a constant size, although V1PieceHeaderReservedArea is
	// constant. Without this marker, we wouldn't have any way to know how much of the
	// reserved header area is supposed to make up the serialized header protobuf.
	var headerBytes [V1PieceHeaderReservedArea]byte
	framingBytes := headerBytes[:v1PieceHeaderFramingSize]
	n, err := io.ReadFull(r.blob, framingBytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if n != v1PieceHeaderFramingSize {
		return nil, Error.New("Could not read whole PieceHeader framing field")
	}
	r.pos += int64(n)
	headerSize := binary.BigEndian.Uint16(framingBytes)
	if headerSize > (V1PieceHeaderReservedArea - v1PieceHeaderFramingSize) {
		return nil, Error.New("PieceHeader framing field claims impossible size of %d bytes", headerSize)
	}

	// Now we can read the actual serialized header.
	pieceHeaderBytes := headerBytes[v1PieceHeaderFramingSize : v1PieceHeaderFramingSize+headerSize]
	n, err = io.ReadFull(r.blob, pieceHeaderBytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	r.pos += int64(n)

	// Deserialize and return.
	header := &pb.PieceHeader{}
	if err := pb.Unmarshal(pieceHeaderBytes, header); err != nil {
		return nil, Error.New("piece header: %w", err)
	}
	return header, nil
}

// Read reads data from the underlying blob, buffering as necessary.
func (r *Reader) Read(data []byte) (int, error) {
	if r.formatVersion >= filestore.FormatV1 && r.pos < V1PieceHeaderReservedArea {
		// should only be necessary once per reader. or zero times, if GetPieceHeader is used
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return 0, Error.Wrap(err)
		}
	}
	n, err := r.blob.Read(data)
	r.pos += int64(n)
	if errors.Is(err, io.EOF) {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Seek seeks to the specified location within the piece content (ignoring the header).
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekStart && r.formatVersion >= filestore.FormatV1 {
		offset += V1PieceHeaderReservedArea
	}
	if whence == io.SeekStart && r.pos == offset {
		return r.pos, nil
	}

	pos, err := r.blob.Seek(offset, whence)
	r.pos = pos
	if r.formatVersion >= filestore.FormatV1 {
		if pos < V1PieceHeaderReservedArea {
			// any position within the file header should show as 0 here
			pos = 0
		} else {
			pos -= V1PieceHeaderReservedArea
		}
	}
	if errors.Is(err, io.EOF) {
		return pos, err
	}
	return pos, Error.Wrap(err)
}

// ReadAt reads data at the specified offset, which is relative to the piece content,
// not the underlying blob. The piece header is not reachable by this method.
func (r *Reader) ReadAt(data []byte, offset int64) (int, error) {
	if r.formatVersion >= filestore.FormatV1 {
		offset += V1PieceHeaderReservedArea
	}
	n, err := r.blob.ReadAt(data, offset)
	if errors.Is(err, io.EOF) {
		return n, err
	}
	return n, Error.Wrap(err)
}

// Size returns the amount of data in the piece.
func (r *Reader) Size() int64 { return r.pieceSize }

// Close closes the reader.
func (r *Reader) Close() error {
	return Error.Wrap(r.blob.Close())
}
