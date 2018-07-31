// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"bytes"
	"testing"
)

func TestStrictlyLessThanThresh(t *testing.T) {
	file := []byte("abcdefghijklmnopqrstuvwxyz")
	ioReader := bytes.NewReader(file)
	p := NewPeekThresholdReader(ioReader)

	isRemote, err := p.IsLargerThan(30)
	if isRemote {
		t.Fatal("expected isRemote to be false")
	}
	if err != nil {
		t.Fatal("expected no err", err)
	}

	outputBuf := make([]byte, 30)
	n, err := p.Read(outputBuf)
	if n != len(file) {
		t.Fatal("expected size of n to equal length of file")
	}
	if !bytes.Equal(outputBuf[:n], file) {
		t.Fatal("expected data in outputBuf to match data in file")
	}
}
func TestStrictlyGreaterThanThresh(t *testing.T) {
	file := []byte("abcdefghijklmnopqrstuvwxyz")
	ioReader := bytes.NewReader(file)
	p := NewPeekThresholdReader(ioReader)

	isRemote, err := p.IsLargerThan(10)
	if !isRemote {
		t.Fatal("expected isRemote to be true")
	}
	if err != nil {
		t.Fatal("expected no err", err)
	}

	outputBuf := make([]byte, 30)
	n, err := p.Read(outputBuf)
	if n != len(file) {
		t.Fatal("expected size of n to equal length of file")
	}
	if !bytes.Equal(outputBuf[:n], file) {
		t.Fatal("expected data in outputBuf to match data in file")
	}
}

func TestReadLessThanThresholdBuf(t *testing.T) {
	file := []byte("abcdefghijklmnopqrstuvwxyz")
	ioReader := bytes.NewReader(file)
	p := NewPeekThresholdReader(ioReader)

	isRemote, err := p.IsLargerThan(20)
	if !isRemote {
		t.Fatal("expected isRemote to be true")
	}
	if err != nil {
		t.Fatal("expected no err", err)
	}

	outputBuf := make([]byte, 10)
	n, err := p.Read(outputBuf)
	if n != len(outputBuf) {
		t.Fatal("expected size of n to equal length of outputBuf")
	}
	if !bytes.Equal(outputBuf, file[:n]) {
		t.Fatal("expected data in outputBuf to match data in beginning of file")
	}
}

func TestMultipleIsLargerCall(t *testing.T) {
	file := []byte("abcdefghijklmnopqrstuvwxyz")
	ioReader := bytes.NewReader(file)
	p := NewPeekThresholdReader(ioReader)

	isRemote, err := p.IsLargerThan(20)
	if !isRemote {
		t.Fatal("expected isRemote to be true")
	}
	if err != nil {
		t.Fatal("expected no err", err)
	}

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
	if err != nil {
		t.Fatal("expected no err from Read")
	}

	_, err = p.IsLargerThan(20)
	if err == nil {
		t.Fatal("expected to err because IsLargerThan called after Read")
	}
}
