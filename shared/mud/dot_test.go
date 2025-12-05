// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"path/filepath"
	"testing"
)

func TestDot(t *testing.T) {
	t.Skip("This test required dot executable")
	dir := t.TempDir()
	ball := NewBall()
	Provide[DB](ball, NewDB)
	Provide[Service1](ball, NewService1)
	Provide[Service2](ball, NewService2)

	// We don't really assert the results, as it may be changed, but it should be executed.
	MustGenerateGraph(ball, filepath.Join(dir, "graph"), All)
}
