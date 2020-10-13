// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package ordersfile

import (
	"encoding/binary"
	"errors"
	"io"
	"os"

	"go.uber.org/zap"

	"storj.io/common/pb"
)

// fileV0 is a version 0 orders file.
type fileV0 struct {
	log *zap.Logger
	f   *os.File
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
		// if error is unexpected EOF, file is corrupted.
		// V0 files do not handle corruption, so just return EOF so caller thinks we have reached the end of the file.
		if errors.Is(err, io.ErrUnexpectedEOF) {
			of.log.Warn("Unexpected EOF while reading archived order file", zap.Error(err))
			mon.Meter("orders_archive_file_corrupted").Mark64(1)
			err = io.EOF
		}
	}()

	sizeBytes := [4]byte{}
	_, err = io.ReadFull(of.f, sizeBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	limitSize := binary.LittleEndian.Uint32(sizeBytes[:])
	limitSerialized := make([]byte, limitSize)
	_, err = io.ReadFull(of.f, limitSerialized)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	limit := &pb.OrderLimit{}
	err = pb.Unmarshal(limitSerialized, limit)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = io.ReadFull(of.f, sizeBytes[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	orderSize := binary.LittleEndian.Uint32(sizeBytes[:])
	orderSerialized := make([]byte, orderSize)
	_, err = io.ReadFull(of.f, orderSerialized)
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

// Close closes the file.
func (of *fileV0) Close() error {
	return of.f.Close()
}
