// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/zeebo/blake3"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

var (
	doFindCorrectLength = flag.Bool("find-length", false,
		"If set, when a file with invalid length is detected, try to determine the correct length by evaluating "+
			"the hash after every byte. This may be slow.")
)

const (
	v1PieceHeaderFramingSize = 2
	// Rather than look up all the details of how a signature digest is encoded with its salt and how big a salt can be,
	// I'll establish some too-loose bounds based purely on observation.
	minSignatureSize = sha256.Size
	maxSignatureSize = 80
)

var (
	wayTooEarly = time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC)
	wayTooLate  = time.Now().AddDate(0, 0, 1)

	pathEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)
)

func readBlobHeader(r io.Reader) (*pb.PieceHeader, error) {
	var headerBytes [pieces.V1PieceHeaderReservedArea]byte
	framingBytes := headerBytes[:v1PieceHeaderFramingSize]
	n, err := io.ReadFull(r, framingBytes)
	if err != nil {
		return nil, err
	}
	if n != v1PieceHeaderFramingSize {
		return nil, errors.New("could not read whole PieceHeader framing field")
	}
	headerSize := binary.BigEndian.Uint16(framingBytes)
	if headerSize > (pieces.V1PieceHeaderReservedArea - v1PieceHeaderFramingSize) {
		return nil, fmt.Errorf("blob PieceHeader framing field claims impossible size of %d bytes", headerSize)
	}

	// Now we can read the actual serialized header.
	pieceHeaderBytes := headerBytes[v1PieceHeaderFramingSize : v1PieceHeaderFramingSize+headerSize]
	_, err = io.ReadFull(r, pieceHeaderBytes)
	if err != nil {
		return nil, err
	}

	// Deserialize and return.
	header := &pb.PieceHeader{}
	if err := pb.Unmarshal(pieceHeaderBytes, header); err != nil {
		return nil, fmt.Errorf("deserializing piece header: %w", err)
	}
	return header, nil
}

func checkHash(r io.Reader, claimedHash []byte, hashAlgo pb.PieceHashAlgorithm) (matches bool, err error) {
	hasher := pb.NewHashFromAlgorithm(hashAlgo)
	if _, err := io.Copy(hasher, r); err != nil {
		return false, err
	}
	calculatedHash := hasher.Sum(nil)
	return bytes.Equal(claimedHash, calculatedHash), nil
}

func checkSanity(header *pb.PieceHeader, fileSize int64) error {
	var expectHashSize int
	switch header.HashAlgorithm {
	case pb.PieceHashAlgorithm_SHA256:
		expectHashSize = sha256.Size
	case pb.PieceHashAlgorithm_BLAKE3:
		expectHashSize = blake3.New().Size()
	default:
		return fmt.Errorf("invalid PieceHashAlgorithm %d", header.HashAlgorithm)
	}
	sig := header.Signature
	if len(sig) < minSignatureSize || len(sig) > maxSignatureSize {
		return fmt.Errorf("signature field has invalid size %d", len(sig))
	}
	sig2 := header.OrderLimit.SatelliteSignature
	if len(sig2) < minSignatureSize || len(sig2) > maxSignatureSize {
		return fmt.Errorf("satellite signature field has invalid size %d", len(sig2))
	}
	if header.OrderLimit.Limit < (fileSize - int64(pieces.V1PieceHeaderReservedArea)) {
		return fmt.Errorf("order limit size %d is too small for file size %d", header.OrderLimit.Limit, fileSize)
	}
	if len(header.Hash) != expectHashSize {
		return fmt.Errorf("hash field should be %d bytes, but is %d bytes", expectHashSize, len(header.Hash))
	}
	if header.OrderLimit.OrderCreation.Before(wayTooEarly) {
		return fmt.Errorf("order creation field has improbably early value %s", header.OrderLimit.OrderCreation.String())
	}
	if header.OrderLimit.OrderCreation.After(wayTooLate) {
		return fmt.Errorf("order creation field has improbably late value %s", header.OrderLimit.OrderCreation.String())
	}
	switch header.OrderLimit.Action {
	case pb.PieceAction_PUT, pb.PieceAction_GET, pb.PieceAction_GET_AUDIT, pb.PieceAction_GET_REPAIR, pb.PieceAction_PUT_REPAIR, pb.PieceAction_DELETE, pb.PieceAction_PUT_GRACEFUL_EXIT:
	default:
		return fmt.Errorf("order limit action has invalid value %d", header.OrderLimit.Action)
	}
	return nil
}

func findCorrectLength(r io.Reader, claimedHash []byte, hashAlgo pb.PieceHashAlgorithm) (rightLength int64, err error) {
	hasher := pb.NewHashFromAlgorithm(hashAlgo)
	var readBytes int64
	calculatedHash := make([]byte, hasher.Size())
	for {
		cHash := hasher.Sum(calculatedHash[:0])
		if bytes.Equal(claimedHash, cHash) {
			return readBytes, nil
		}
		var buf [1]byte
		n, err := r.Read(buf[:])
		if n == 1 {
			// we do this even if err != nil; i.e. if err = io.EOF, the read byte is still valid
			hasher.Write(buf[:])
			readBytes++
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, errors.New("no possible valid length found. (possibly the blob is truncated?)")
			}
			return 0, err
		}
	}
}

func checkFile(fileName string) (report string, realFilename string, err error) {
	fh, err := os.Open(fileName)
	if err != nil {
		return "", "", fmt.Errorf("could not open file %s: %w", fileName, err)
	}
	header, err := readBlobHeader(fh)
	if err != nil {
		return fmt.Sprintf("not a valid sj1 blob (%v)", err), "", nil
	}
	if header.FormatVersion != 1 {
		return fmt.Sprintf("not a valid sj1 blob (FormatVersion=%d)", header.FormatVersion), "", nil
	}
	fileSize, err := fh.Seek(0, io.SeekEnd)
	if err != nil {
		return "", "", fmt.Errorf("could not seek to end of file: %w", err)
	}
	_, err = fh.Seek(pieces.V1PieceHeaderReservedArea, io.SeekStart)
	if err != nil {
		return "", "", fmt.Errorf("could not seek to after header area: %w", err)
	}
	err = checkSanity(header, fileSize)
	if err != nil {
		return fmt.Sprintf("not a valid sj1 blob (%v)", err), "", nil
	}
	matches, err := checkHash(fh, header.Hash, header.HashAlgorithm)
	if err != nil {
		return "", "", fmt.Errorf("could not read file data to check hash: %w", err)
	}
	realFilename = v1FilenameFor(header.OrderLimit.SatelliteId, header.OrderLimit.PieceId)
	if matches {
		return "valid sj1 blob", realFilename, nil
	}
	if *doFindCorrectLength {
		_, err = fh.Seek(pieces.V1PieceHeaderReservedArea, io.SeekStart)
		if err != nil {
			return "", "", fmt.Errorf("could not seek to after header area: %w", err)
		}
		rightLength, err := findCorrectLength(fh, header.Hash, header.HashAlgorithm)
		if err != nil {
			return fmt.Sprintf("appears to be a valid sj1 blob, but could not determine correct length: %v", err), "", nil
		}
		fileLength := rightLength + pieces.V1PieceHeaderReservedArea
		return fmt.Sprintf("valid sj1 blob but should be truncated at %d bytes. Hint:\n    truncate -s %d %q", fileLength, fileLength, fileName), realFilename, nil
	}
	return "valid-looking sj1 blob with hash mismatch", realFilename, nil
}

func v1FilenameFor(satelliteID storj.NodeID, pieceID storj.PieceID) string {
	pieceIDEncoded := pathEncoding.EncodeToString(pieceID[:])
	satelliteIDEncoded := pathEncoding.EncodeToString(satelliteID[:])
	return fmt.Sprintf("%s/%s/%s.sj1", satelliteIDEncoded, pieceIDEncoded[0:2], pieceIDEncoded[2:])
}

func main() {
	flag.Parse()

	fileNames := flag.Args()
	for _, name := range fileNames {
		report, realFilename, err := checkFile(name)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			continue
		}
		fmt.Printf("%s: %s\n", name, report)
		if realFilename != "" {
			fmt.Printf("%s=%s\n", name, realFilename)
		}
	}
}
