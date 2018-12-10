// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaxRead(t *testing.T) {
	for _, tt := range []struct {
		name         string
		file         []byte
		outputBufLen int
		maxSize      int64
		n            int
		eofErr       bool
		exceedErr    bool
	}{
		{"Test io.Reader UnexpectedEOF error: ", []byte("abcdefghijklmnopqrstuvwxyz"), 30, 40, 26, true, false},
		{"Test exceeds max error: ", []byte("abcdefghijklmnopqrstuvwxyz"), 30, 5, 26, false, true},
		{"Test no error: ", []byte("abcdefghijklmnopqrstuvwxyz"), 20, 40, 20, false, false},
	} {
		ioReader := bytes.NewReader(tt.file)
		mr := NewMaxReader(ioReader, tt.maxSize)

		outputBuf := make([]byte, tt.outputBufLen)
		n, err := io.ReadFull(mr, outputBuf)

		if tt.eofErr {
			assert.Error(t, err)
			assert.True(t, err == io.ErrUnexpectedEOF)
		} else if tt.exceedErr {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "Data read from reader exceeds maximum allowed")
		} else {
			assert.NoError(t, err)
		}

		assert.Equal(t, n, tt.n)
	}
}
