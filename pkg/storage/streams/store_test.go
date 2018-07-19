// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"testing"
	"time"

	"storj.io/storj/pkg/paths"
)

func TestStreamPut(t *testing.T) {
	ctx := context.Background()
	path := paths.New("test")
	data := trings.NewReader("Test Str")
	var metadata []byte
	expiration := time.Now()

	segment := NewSegmentStore()
}
