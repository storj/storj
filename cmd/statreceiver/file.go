// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"encoding/gob"
	"io"
	"os"
	"time"
)

type FileSource struct {
	path    string
	decoder *gob.Decoder
}

func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

func (f *FileSource) Next() ([]byte, time.Time, error) {
	if f.decoder == nil {
		fh, err := os.Open(f.path)
		if err != nil {
			return nil, time.Time{}, err
		}
		f.decoder = gob.NewDecoder(bufio.NewReader(fh))
	}

	var p Packet
	err := f.decoder.Decode(&p)
	if err != nil {
		return nil, time.Time{}, err
	}
	return p.Data, p.TS, nil
}

type FileDest struct {
	path    string
	fh      io.Closer
	encoder *gob.Encoder
}

func NewFileDest(path string) *FileDest {
	return &FileDest{path: path}
}

func (f *FileDest) Packet(data []byte, ts time.Time) error {
	if f.encoder == nil {
		fh, err := os.Create(f.path)
		if err != nil {
			return err
		}
		f.fh = fh
		f.encoder = gob.NewEncoder(bufio.NewWriter(fh))
	}
	return f.encoder.Encode(Packet{Data: data, TS: ts})
}
