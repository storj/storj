// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package lifecycle

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCondenseStack(t *testing.T) {
	buf := make([]byte, 1024*1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}

	condensed := condenseStack(buf)

	t.Logf("uncondensed:\n%s", buf)
	t.Logf("condensed:\n%s", condensed)

	require.Less(t, len(condensed), len(buf))
}
