// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/storj"
)

func TestSerialNumber_Encode(t *testing.T) {
	_, err := storj.SerialNumberFromString("likn43kilfzd")
	assert.Error(t, err)

	_, err = storj.SerialNumberFromBytes([]byte{1, 2, 3, 4, 5})
	assert.Error(t, err)

	for i := 0; i < 10; i++ {
		var serialNumber storj.SerialNumber
		_, _ = rand.Read(serialNumber[:])

		fromString, err := storj.SerialNumberFromString(serialNumber.String())
		assert.NoError(t, err)
		fromBytes, err := storj.SerialNumberFromBytes(serialNumber.Bytes())
		assert.NoError(t, err)

		assert.Equal(t, serialNumber, fromString)
		assert.Equal(t, serialNumber, fromBytes)
	}
}
