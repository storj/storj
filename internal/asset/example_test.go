// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset_test

import (
	"testing"

	"storj.io/storj/internal/asset"
)

func TestDir(t *testing.T) {
	root, err := asset.NewDir(".")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(root.GenerateGo())
}
