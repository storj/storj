// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ranger

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRange(t *testing.T) {
	for i, tt := range []struct {
		offset int64
		length int64
		size   int64
	}{
		{offset: -2, size: 0},
		{offset: 2, length: -1, size: 0},
	} {
		tag := fmt.Sprintf("#%d. %+v", i, tt)

		rr := readerAtRanger{size: tt.size}
		closer, err := rr.Range(context.Background(), tt.offset, tt.length)
		assert.Nil(t, closer, tag)
		assert.NotNil(t, err, tag)
	}
}

func TestClose(t *testing.T) {
	rr := readerAtReader{length: 0}
	assert.Nil(t, rr.Close())
}
