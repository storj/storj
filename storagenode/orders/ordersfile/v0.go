// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package ordersfile

import (
	"encoding/binary"
	"errors"
	"io"
	"os"

	"storj.io/common/pb"
)

// fileV0 is a version 0 orders file.
type fileV0 struct {
	f *os.File
}

// OpenWritableV0 opens for writing the unsent or archived orders file at a given path.
func OpenWritableV0(path string) (Writable, error) {
	// create file if not exists or append
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &fileV0{
		f: f,
	}, nil
}

// OpenReadableV0 opens for reading the unsent or archived orders file at a given path.
func OpenReadableV0(path string) (Readable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &fileV0{
		f: f,
	}, nil
}

// Append writes limit and order to the file as
// [limitSize][limitBytes][orderSize][orderBytes].
func (of *fileV0) Append(info *Info) error {
	toWrite := []byte{}

	limitSerialized, err := pb.Marshal(info.Limit)
	if err != nil {
		return Error.Wrap(err)
	}
	orderSerialized, err := pb.Marshal(info.Order)
	if err != nil {
		return Error.Wrap(err)
	}

	limitSizeBytes := [4]byte{}
	binary.LittleEndian.PutUint32(limitSizeBytes[:], uint32(len(limitSerialized)))

	orderSizeBytes := [4]byte{}
	binary.LittleEndian.PutUint32(orderSizeBytes[:], uint32(len(orderSerialized)))

	toWrite = append(toWrite, limitSizeBytes[:]...)
	toWrite = append(toWrite, limitSerialized...)
	toWrite = append(toWrite, orderSizeBytes[:]...)
	toWrite = append(toWrite, orderSerialized...)

	if _, err = of.f.Write(toWrite); err != nil {
		return Error.New("Couldn't write serialized order and limit: %w", err)
	}

	return nil
}

// ReadOne reads one entry from the file.
func (of *fileV0) ReadOne() (info *Info, err error) {
	defer func() {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			err = ErrEntryCorrupt.Wrap(err)
		}
	}()

	sizeBytes := [4]byte{}
	_, err = io.ReadFull(of.f, sizeBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	limitSize := binary.LittleEndian.Uint32(sizeBytes[:])
	if limitSize > uint32(orderLimitSizeCap) {
		return nil, ErrEntryCorrupt.New("invalid limit size: %d is over the maximum %d", limitSize, orderLimitSizeCap)
	}

	limitSerialized := make([]byte, limitSize)
	_, err = io.ReadFull(of.f, limitSerialized)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	limit := &pb.OrderLimit{}
	err = pb.Unmarshal(limitSerialized, limit)
	if err != nil {
		// if there is an error unmarshalling, the file must be corrupt
		return nil, ErrEntryCorrupt.Wrap(err)
	}

	_, err = io.ReadFull(of.f, sizeBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	orderSize := binary.LittleEndian.Uint32(sizeBytes[:])
	if orderSize > uint32(orderSizeCap) {
		return nil, ErrEntryCorrupt.New("invalid order size: %d is over the maximum %d", orderSize, orderSizeCap)
	}

	orderSerialized := make([]byte, orderSize)
	_, err = io.ReadFull(of.f, orderSerialized)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	order := &pb.Order{}
	err = pb.Unmarshal(orderSerialized, order)
	if err != nil {
		// if there is an error unmarshalling, the file must be corrupt
		return nil, ErrEntryCorrupt.Wrap(err)
	}

	return &Info{
		Limit: limit,
		Order: order,
	}, nil
}

// Close closes the file.
func (of *fileV0) Close() error {
	return of.f.Close()
}
