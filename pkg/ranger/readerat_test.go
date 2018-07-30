// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"context"
)

func TestRange(t *testing.T) {
	for _, tt := range []struct {
		name   string
		offset int64
		length int64
		size   int64
	}{
		{
			name:   "Negative offset",
			offset: -2,
		},

		{
			name:   "Negative length",
			offset: 2,
			length: -1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rr := readerAtRanger{size: tt.size}
			closer, err := rr.Range(context.Background(), tt.offset, tt.length)
			assert.Nil(t, closer)
			assert.NotNil(t, err)
		})
	}
}

func TestClose(t *testing.T) {
	rr := readerAtReader{length:0}
	assert.Nil(t, rr.Close())
}