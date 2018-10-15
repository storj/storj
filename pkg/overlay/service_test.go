// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"testing"
)

func TestNewServerNilArgs(t *testing.T) {
	server := NewServer(nil, nil, nil, nil)
	if server == nil {
		t.Fatal("got nil server")
	}
}
