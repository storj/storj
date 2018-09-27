// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import "testing"

func TestDirInfoFromPath(t *testing.T) {
	filesytemID, amount, err := diskInfoFromPath(".")
	if err != nil {
		t.Fatal(err)
	}
	if amount <= 0 {
		t.Fatal("expected to have some disk space")
	}
	if filesytemID == "" {
		t.Fatal("didn't get filesystem id")
	}

	t.Logf("Got: %v %v", filesytemID, amount)
}
