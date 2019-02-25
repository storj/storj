// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import "testing"

func TestDiskInfoFromPath(t *testing.T) {
	info, err := diskInfoFromPath(".")
	if err != nil {
		t.Fatal(err)
	}
	if info.AvailableSpace <= 0 {
		t.Fatal("expected to have some disk space")
	}
	if info.ID == "" {
		t.Fatal("didn't get filesystem id")
	}

	t.Logf("Got: %v %v", info.ID, info.AvailableSpace)
}
