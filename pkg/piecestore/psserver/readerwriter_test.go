// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psserver

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/utils"
)

func TestRead(t *testing.T) {
	for _, tt := range []struct {
		name         string
		file         []byte
		outputBufLen int
		bwLeft       int64
		spaceLeft    int64
		n            int
		eofErr       bool
		bwErr        bool
		spaceErr     bool
	}{
		{"Test EOF error: ", []byte("abcdefghijklmnopqrstuvwxyz"), 30, 40, 40, 26, true, false, false},
		{"Test exceeds bandwidth error: ", []byte("abcdefghijklmnopqrstuvwxyz"), 26, 5, 40, 10, false, true, false},
		{"Test exceeds space error 1: ", []byte("abcdefghijklmnopqrstuvwxyz"), 26, 40, 5, 10, false, false, true},
		{"Test exceeds space error 2: ", []byte("abcdefghijklmnopqrstuvwxyz"), 26, 40, 0, 0, false, false, true},
		{"Test no error: ", []byte("abcdefghijklmnopqrstuvwxyz"), 20, 40, 40, 20, false, false, false},
	} {
		remaining := tt.file
		readerSrc := utils.NewReaderSource(func() ([]byte, error) {
			if len(remaining) == 0 {
				return nil, io.EOF
			}

			// send in 10 byte chunks
			if len(remaining) <= 10 {
				ret := remaining
				remaining = []byte{}
				return ret, io.EOF
			}

			ret := remaining[:10]
			remaining = remaining[10:]

			return ret, nil
		})
		sr := &StreamReader{
			src:                readerSrc,
			bandwidthRemaining: tt.bwLeft,
			spaceRemaining:     tt.spaceLeft,
		}

		outputBuf := make([]byte, tt.outputBufLen)
		n, err := io.ReadFull(sr, outputBuf)

		if tt.eofErr {
			assert.Error(t, err)
			assert.True(t, err == io.ErrUnexpectedEOF)
		} else if tt.bwErr {
			assert.Error(t, err)
			assert.True(t, StreamWriterError.Has(err))
			assert.Contains(t, err.Error(), "out of bandwidth")
		} else if tt.spaceErr {
			assert.Error(t, err)
			assert.True(t, StreamWriterError.Has(err))
			assert.Contains(t, err.Error(), "out of space")
		} else {
			assert.NoError(t, err)
		}

		assert.Equal(t, n, tt.n)
	}
}
