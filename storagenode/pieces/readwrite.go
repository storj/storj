// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"encoding/binary"
	"hash"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/pkcrypto"
	"storj.io/common/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
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

// BadFormatVersion is returned when a storage format cannot support the request function
var BadFormatVersion = errs.Class("Incompatible storage format version")

// Writer implements a piece writer that writes content to blob store and calculates a hash.
type Writer struct {
	hash      hash.Hash
	blob      storage.BlobWriter
	pieceSize int64 // piece size only; i.e., not including piece header

	blobs     storage.Blobs
	satellite storj.NodeID
	closed    bool
}

// NewWriter creates a new writer for storage.BlobWriter.
func NewWriter(blobWriter storage.BlobWriter, blobs storage.Blobs, satellite storj.NodeID) (*Writer, error) {
	w := &Writer{}
	if blobWriter.StorageFormatVersion() >= filestore.FormatV1 {
		// We skip past the reserved header area for now- we want the header to be at the
		// beginning of the file, to make it quick to seek there and also to make it easier
		// to identify situations where a blob file has been truncated incorrectly. And we
		// don't know what exactly is going to be in the header yet--we won't know what the
		// hash or size or timestamp or expiration or signature fields need to be until we
		// have received the whole piece.
		//
		// Once the writer calls Commit() on this writer, we will seek back to the beginning
		// of the file and write the header.
		if _, err := blobWriter.Seek(V1PieceHeaderReservedArea, io.SeekStart); err != nil {
			return nil, Error.Wrap(err)
		}
	}
	w.blob = blobWriter
	w.hash = pkcrypto.NewHash()
	w.blobs = blobs
	w.satellite = satellite
	return w, nil
}

// Write writes data to the blob and calculates the hash.
func (w *Writer) Write(data []byte) (int, error) {
	n, err := w.blob.Write(data)
	w.pieceSize += int64(n)
	_, _ = w.hash.Write(data[:n]) // guaranteed not to return an error
	if err == io.EOF {
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
	if cache, ok := w.blobs.(*BlobsUsageCache); ok {
		cache.Update(ctx, w.satellite, w.Size(), 0)
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

	formatVer := w.blob.StorageFormatVersion()
	if formatVer == filestore.FormatV0 {
		return nil
	}
	pieceHeader.FormatVersion = pb.PieceHeader_FormatVersion(formatVer)
	headerBytes, err := proto.Marshal(pieceHeader)
	if err != nil {
		return err
	}
	mon.IntVal("storagenode_pieces_pieceheader_size").Observe(int64(len(headerBytes)))
	if len(headerBytes) > (V1PieceHeaderReservedArea - v1PieceHeaderFramingSize) {
		// This should never happen under normal circumstances, and it might deserve a panic(),
		// but I'm not *entirely* sure this case can't be triggered by a malicious uplink. Are
		// google.protobuf.Timestamp fields variable-width?
		mon.Meter("storagenode_pieces_pieceheader_overflow").Mark(len(headerBytes))
		return Error.New("marshaled piece header too big!")
	}
	size, err := w.blob.Size()
	if err != nil {
		return err
	}
	if _, err := w.blob.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// We need to store some "framing" bytes first, because protobufs are not self-delimiting.
	// In cases where the serialized pieceHeader is not exactly V1PieceHeaderReservedArea bytes
	// (probably _all_ cases), without this marker, we wouldn't have any way to take the
	// V1PieceHeaderReservedArea bytes from a piece blob and trim off the right number of zeroes
	// at the end so that the protobuf unmarshals correctly.
	var framingBytes [v1PieceHeaderFramingSize]byte
	binary.BigEndian.PutUint16(framingBytes[:], uint16(len(headerBytes)))
	if _, err = w.blob.Write(framingBytes[:]); err != nil {
		return Error.New("failed writing piece framing field at file start: %v", err)
	}

	// Now write the serialized header bytes.
	if _, err = w.blob.Write(headerBytes); err != nil {
		return Error.New("failed writing piece header at file start: %v", err)
	}

	// seek back to the end, as blob.Commit will truncate from the current file position.
	// (don't try to seek(0, io.SeekEnd), because dir.CreateTemporaryFile preallocs space
	// and the actual end of the file might be far past the intended end of the piece.)
	if _, err := w.blob.Seek(size, io.SeekStart); err != nil {
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

	blob      storage.BlobReader
	pos       int64 // relative to file start; i.e., it includes piece header
	pieceSize int64 // piece size only; i.e., not including piece header
}

// NewReader creates a new reader for storage.BlobReader.
func NewReader(blob storage.BlobReader) (*Reader, error) {
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
func (r *Reader) StorageFormatVersion() storage.FormatVersion {
	return r.formatVersion
}

// GetPieceHeader reads, unmarshals, and returns the piece header. It may only be called once,
// before any Read() calls. (Retrieving the header at any time could be supported, but for the sake
// of performance we need to understand why and how often that would happen.)
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
	if err := proto.Unmarshal(pieceHeaderBytes, header); err != nil {
		return nil, Error.New("piece header: %v", err)
	}
	return header, nil
}

// Read reads data from the underlying blob, buffering as necessary.
func (r *Reader) Read(data []byte) (int, error) {
	if r.formatVersion >= filestore.FormatV1 && r.pos < V1PieceHeaderReservedArea {
		// should only be necessary once per reader. or zero times, if GetPieceHeader is used
		if _, err := r.blob.Seek(V1PieceHeaderReservedArea, io.SeekStart); err != nil {
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
	if err == io.EOF {
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
	if err == io.EOF {
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
