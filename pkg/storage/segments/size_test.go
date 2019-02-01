// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSizeReader(t *testing.T) {
	data := "abcdefgh"
	r := bytes.NewReader([]byte(data))
	sr := SizeReader(r)

	// Nothing has been read yet - size is 0
	assert.EqualValues(t, 0, sr.Size())

	// Read 2 bytes - Size is now 2
	buf := make([]byte, 2)
	_, err := io.ReadFull(sr, buf)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, sr.Size())

	// Read 2 bytes again - Size is now 4
	_, err = io.ReadFull(sr, buf)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, sr.Size())

	// Read all the rest - Size is now len(data)
	_, err = io.Copy(ioutil.Discard, sr)
	assert.NoError(t, err)
	assert.EqualValues(t, len(data), sr.Size())
}
