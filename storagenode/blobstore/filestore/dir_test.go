// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filestore

import (
	"fmt"
	"os"
	"testing"
)

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

func BenchmarkDiskInfoFromPath(b *testing.B) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		b.Fatal(err)
	}
	b.Run(fmt.Sprintf("dir=%q", homedir), func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err = diskInfoFromPath(homedir)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
