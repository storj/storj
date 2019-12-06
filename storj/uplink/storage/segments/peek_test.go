// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThresholdBufAndRead(t *testing.T) {
	for _, tt := range []struct {
		name             string
		file             []byte
		thresholdSize    int
		expectedIsRemote bool
		outputBufLen     int
		readsToEnd       bool
	}{
		{"Test strictly less than threshold: ", []byte("abcdefghijklmnopqrstuvwxyz"), 30, false, 30, true},
		{"Test strictly greater than threshold: ", []byte("abcdefghijklmnopqrstuvwxyz"), 10, true, 30, true},
		{"Test read less than threshold buf: ", []byte("abcdefghijklmnopqrstuvwxyz"), 20, true, 10, false},
		{"Test empty file: ", []byte(""), 10, false, 10, true},
		{"Test threshold size == len(file): ", []byte("abcdefghijklmnopqrstuvwxyz"), 26, false, 26, true},
	} {
		ioReader := bytes.NewReader(tt.file)
		p := NewPeekThresholdReader(ioReader)

		isRemote, err := p.IsLargerThan(tt.thresholdSize)
		assert.Equal(t, tt.expectedIsRemote, isRemote, tt.name)
		assert.NoError(t, err, tt.name)

		outputBuf := make([]byte, tt.outputBufLen)
		n, err := io.ReadFull(p, outputBuf)
		if tt.readsToEnd && err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			t.Fatalf(tt.name, "unexpected err from ReadFull:\n%s", err.Error())
		} else if !tt.readsToEnd && err != nil {
			t.Fatalf(tt.name, "unexpected err from ReadFull:\n%s", err.Error())
		}
		if tt.readsToEnd && n != len(tt.file) {
			t.Fatalf(tt.name, "expected size of n to equal length of file")
		}
		if !tt.readsToEnd && n != tt.outputBufLen {
			t.Fatalf(tt.name, "expected n to equal length of outputBuf")
		}
		if !bytes.Equal(outputBuf[:n], tt.file[:n]) {
			t.Fatalf(tt.name, "expected data in outputBuf to match data in file")
		}
	}
}

func TestMultipleIsLargerCall(t *testing.T) {
	file := []byte("abcdefghijklmnopqrstuvwxyz")
	ioReader := bytes.NewReader(file)
	p := NewPeekThresholdReader(ioReader)

	_, err := p.IsLargerThan(20)
	assert.NoError(t, err)

	_, err = p.IsLargerThan(20)
	if err == nil {
		t.Fatal("expected to err because multiple call")
	}
}

func TestIsLargerThanCalledAfterRead(t *testing.T) {
	file := []byte("abcdefghijklmnopqrstuvwxyz")
	ioReader := bytes.NewReader(file)
	p := NewPeekThresholdReader(ioReader)

	outputBuf := make([]byte, 10)
	_, err := p.Read(outputBuf)
	assert.NoError(t, err)

	_, err = p.IsLargerThan(20)
	if err == nil {
		t.Fatal("expected to err because IsLargerThan called after Read")
	}
}
