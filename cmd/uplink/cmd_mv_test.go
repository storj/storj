// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestMv(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("sj://b1/file1.txt", "remote"),

		ultest.WithFile("/home/user/file1.txt", "local"),
		ultest.WithBucket("b2"),
	)

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "mv", "sj://b1/file1.txt", "sj://b1/moved-file1.txt").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b1/moved-file1.txt", Contents: "remote"},
		)

		state.Succeed(t, "mv", "sj://b1/file1.txt", "sj://b1/prefix/").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b1/prefix/file1.txt", Contents: "remote"},
		)

		state.Succeed(t, "mv", "/home/user/file1.txt", "/home/user/moved-file1.txt").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/moved-file1.txt", Contents: "local"},
		)

		state.Fail(t, "mv", "sj://user/not-existing", "sj://user/moved-file1.txt")
		state.Fail(t, "mv", "/home/user/not-existing", "/home/user/moved-file1.txt")
		state.Fail(t, "mv", "/home/user/file1.txt", "/home/user/file1.txt")
	})

	t.Run("BucketToBucket", func(t *testing.T) {
		state.Succeed(t, "mv", "sj://b1/file1.txt", "sj://b2/file1.txt").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b2/file1.txt", Contents: "remote"},
		)
	})

	t.Run("Relative", func(t *testing.T) {
		state.Fail(t, "mv", "sj://b1/file1.txt", "")
		state.Fail(t, "mv", "", "sj://b1/moved-file1.txt")

		state.Fail(t, "mv", "/home/user/file1.txt", "")
		state.Fail(t, "mv", "", "/home/user/moved-file1.txt")
	})

	t.Run("Mixed", func(t *testing.T) {
		state.Fail(t, "mv", "sj://user/file1.txt", "/home/user/file1.txt")
		state.Fail(t, "mv", "/home/user/file1.txt", "sj://user/file1.txt")
	})
}

func TestMvRecursive(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("sj://b1/file1.txt", "remote"),
		ultest.WithFile("sj://b1/foo/file2.txt", "remote"),
		ultest.WithFile("sj://b1/foo/file3.txt", "remote"),

		ultest.WithFile("/home/user/file1.txt", "local"),
		ultest.WithBucket("b2"),
	)

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "mv", "sj://b1/", "sj://b1/prefix/", "--recursive").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b1/prefix/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/prefix/foo/file2.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/prefix/foo/file3.txt", Contents: "remote"},
		)

		state.Succeed(t, "mv", "sj://b1/prefix/", "sj://b1/", "--recursive").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b1/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/foo/file2.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/foo/file3.txt", Contents: "remote"},
		)

		state.Succeed(t, "mv", "sj://b1/foo/", "sj://b1/foo2/", "--recursive").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b1/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/foo2/file2.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/foo2/file3.txt", Contents: "remote"},
		)

		state.Fail(t, "mv", "sj://b1/foo", "sj://b1/foo2", "--recursive")
		state.Fail(t, "mv", "sj://b1/foo", "sj://b1/foo2/", "--recursive")
		state.Fail(t, "mv", "sj://b1/foo/", "sj://b1/foo2", "--recursive")
		state.Fail(t, "mv", "sj://b1/", "/home/user/", "--recursive")
		state.Fail(t, "mv", "/home/user/", "sj://user/", "--recursive")
	})

	t.Run("BucketToBucket", func(t *testing.T) {
		state.Succeed(t, "mv", "sj://b1/", "sj://b2/", "--recursive").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b2/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b2/foo/file2.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b2/foo/file3.txt", Contents: "remote"},
		)
	})

	t.Run("Parallelism", func(t *testing.T) {
		state.Succeed(t, "mv", "sj://b1/", "sj://b1/prefix/", "--recursive", "--parallelism", "2").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://b1/prefix/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/prefix/foo/file2.txt", Contents: "remote"},
			ultest.File{Loc: "sj://b1/prefix/foo/file3.txt", Contents: "remote"},
		)

		state.Fail(t, "mv", "sj://b1/", "sj://b1/prefix/", "--recursive", "--parallelism", "0")
	})
}
