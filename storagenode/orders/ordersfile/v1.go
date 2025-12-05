// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package ordersfile

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
)

var (
	// fileMagic used to identify header of file.
	// "0ddba11 acc01ade5".
	fileMagic = [8]byte{0x0d, 0xdb, 0xa1, 0x1a, 0xcc, 0x01, 0xad, 0xe5}

	// entryHeader is 8 bytes that appears before every order/limit in a V1 file.
	// "5ca1ab1e ba5eba11".
	entryHeader = [8]byte{0x5c, 0xa1, 0xab, 0x1e, 0xba, 0x5e, 0xba, 0x11}
	// entryFooter is 8 bytes that appears after every order/limit in a V1 file.
	// "feed 1 f00d 1 c0ffee".
	entryFooter = [8]byte{0xfe, 0xed, 0x1f, 0x00, 0xd1, 0xc0, 0xff, 0xee}
)

// fileV1 is a version 1 orders file.
type fileV1 struct {
	f  *os.File
	br *bufio.Reader
}

// OpenWritableV1 opens for writing the unsent or archived orders file at a given path.
// If the file is new, the file header is written.
func OpenWritableV1(path string, satelliteID storj.NodeID, creationTime time.Time) (Writable, error) {
	// create file if not exists or append
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	of := &fileV1{
		f: f,
	}

	currentPos, err := of.f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if currentPos == 0 {
		err = of.writeHeader(satelliteID, creationTime)
		if err != nil {
			return of, err
		}
	}

	return of, nil
}

// writeHeader writes file header as [filemagic][satellite ID][creation hour].
func (of *fileV1) writeHeader(satelliteID storj.NodeID, creationTime time.Time) error {
	toWrite := fileMagic[:]
	toWrite = append(toWrite, satelliteID.Bytes()...)
	creationHour := date.TruncateToHourInNano(creationTime)
	creationHourBytes := [8]byte{}
	binary.LittleEndian.PutUint64(creationHourBytes[:], uint64(creationHour))
	toWrite = append(toWrite, creationHourBytes[:]...)

	if _, err := of.f.Write(toWrite); err != nil {
		return Error.New("Couldn't write file header: %w", err)
	}

	return nil
}

// OpenReadableV1 opens for reading the unsent or archived orders file at a given path.
func OpenReadableV1(path string) (Readable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &fileV1{
		f: f,
		// buffered reader is used to search for entryHeader that precedes limit and order.
		br: bufio.NewReader(f),
	}, nil
}

// Append writes limit and order to the file as
// [entryHeader][limitSize][limitBytes][orderSize][orderBytes][checksum][entryFooter].
func (of *fileV1) Append(info *Info) error {
	toWrite := entryHeader[:]

	limitSerialized, err := pb.Marshal(info.Limit)
	if err != nil {
		return Error.Wrap(err)
	}
	orderSerialized, err := pb.Marshal(info.Order)
	if err != nil {
		return Error.Wrap(err)
	}

	limitSizeBytes := [2]byte{}
	binary.LittleEndian.PutUint16(limitSizeBytes[:], uint16(len(limitSerialized)))

	orderSizeBytes := [2]byte{}
	binary.LittleEndian.PutUint16(orderSizeBytes[:], uint16(len(orderSerialized)))

	toWrite = append(toWrite, limitSizeBytes[:]...)
	toWrite = append(toWrite, limitSerialized...)
	toWrite = append(toWrite, orderSizeBytes[:]...)
	toWrite = append(toWrite, orderSerialized...)
	checksumInt := crc32.ChecksumIEEE(toWrite[len(entryHeader):])

	checksumBytes := [4]byte{}
	binary.LittleEndian.PutUint32(checksumBytes[:], checksumInt)
	toWrite = append(toWrite, checksumBytes[:]...)

	toWrite = append(toWrite, entryFooter[:]...)

	if _, err = of.f.Write(toWrite); err != nil {
		return Error.New("Couldn't write serialized order and limit: %w", err)
	}

	return nil
}

// ReadOne reads one entry from the file.
// It returns ErrEntryCorrupt upon finding a corrupt limit/order combo. On next call after a corrupt entry, it will find the next valid order.
func (of *fileV1) ReadOne() (info *Info, err error) {
	// attempt to read an order/limit; if corrupted, keep trying until EOF or uncorrupted pair found.
	// start position will be the position of the of.f cursor minus the number of unread buffered bytes in of.br
	startPosition, err := of.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	startPosition -= int64(of.br.Buffered())

	defer func() {
		// Treat all non-EOF errors as corrupt entry errors so that ReadOne is called again.
		if err != nil && !errors.Is(err, io.EOF) {
			// seek forward by len(entryHeader) bytes for next iteration.
			nextStartPosition := startPosition + int64(len(entryHeader))
			_, seekErr := of.f.Seek(nextStartPosition, io.SeekStart)
			if seekErr != nil {
				err = errs.Combine(err, seekErr)
			}
			of.br.Reset(of.f)

			err = ErrEntryCorrupt.Wrap(err)
		}
	}()

	err = of.gotoNextEntry()
	if err != nil {
		return nil, err
	}

	limitSizeBytes := [2]byte{}
	_, err = io.ReadFull(of.br, limitSizeBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	limitSize := binary.LittleEndian.Uint16(limitSizeBytes[:])
	if limitSize > uint16(orderLimitSizeCap) {
		return nil, ErrEntryCorrupt.New("invalid limit size: %d is over the maximum %d", limitSize, orderLimitSizeCap)
	}

	limitSerialized := make([]byte, limitSize)
	_, err = io.ReadFull(of.br, limitSerialized)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	orderSizeBytes := [2]byte{}
	_, err = io.ReadFull(of.br, orderSizeBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	orderSize := binary.LittleEndian.Uint16(orderSizeBytes[:])
	if orderSize > uint16(orderSizeCap) {
		return nil, ErrEntryCorrupt.New("invalid order size: %d is over the maximum %d", orderSize, orderSizeCap)
	}
	orderSerialized := make([]byte, orderSize)
	_, err = io.ReadFull(of.br, orderSerialized)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// read checksum
	checksumBytes := [4]byte{}
	_, err = io.ReadFull(of.br, checksumBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	expectedChecksum := binary.LittleEndian.Uint32(checksumBytes[:])

	actualChecksum := uint32(0)
	actualChecksum = crc32.Update(actualChecksum, crc32.IEEETable, limitSizeBytes[:])
	actualChecksum = crc32.Update(actualChecksum, crc32.IEEETable, limitSerialized)
	actualChecksum = crc32.Update(actualChecksum, crc32.IEEETable, orderSizeBytes[:])
	actualChecksum = crc32.Update(actualChecksum, crc32.IEEETable, orderSerialized)

	if expectedChecksum != actualChecksum {
		return nil, Error.New("checksum does not match")
	}

	footerBytes := [len(entryFooter)]byte{}
	_, err = io.ReadFull(of.br, footerBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if !bytes.Equal(entryFooter[:], footerBytes[:]) {
		return nil, Error.New("footer bytes do not match")
	}

	limit := &pb.OrderLimit{}
	err = pb.Unmarshal(limitSerialized, limit)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	order := &pb.Order{}
	err = pb.Unmarshal(orderSerialized, order)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &Info{
		Limit: limit,
		Order: order,
	}, nil
}

func (of *fileV1) gotoNextEntry() error {
	// search file for next occurrence of entry header, or until EOF
	for {
		searchBufSize := 2 * memory.KiB.Int()
		nextBufferBytes, err := of.br.Peek(searchBufSize)
		// if the buffered reader hits an EOF, the buffered data may still
		// contain a full entry, so do not return unless there is definitely no entry
		if errors.Is(err, io.EOF) && len(nextBufferBytes) <= len(entryHeader) {
			return err
		} else if err != nil && !errors.Is(err, io.EOF) {
			return Error.Wrap(err)
		}

		i := bytes.Index(nextBufferBytes, entryHeader[:])
		if i > -1 {
			_, err = of.br.Discard(i + len(entryHeader))
			if err != nil {
				return Error.Wrap(err)
			}
			break
		}
		// entry header not found; discard all but last (len(entryHeader)-1) bytes for next iteration
		_, err = of.br.Discard(len(nextBufferBytes) - len(entryHeader) + 1)
		if err != nil {
			return Error.Wrap(err)
		}
	}
	return nil
}

// Close closes the file.
func (of *fileV1) Close() error {
	return of.f.Close()
}
