// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"storj.io/storj/cmd/uplink/ultest"
)

func TestRmRemote(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		state := ultest.Setup(commands,
			ultest.WithFile("sj://user/file1.txt"),
			ultest.WithFile("sj://user/file2.txt"),
			ultest.WithFile("/home/user/file1.txt"),
			ultest.WithFile("/home/user/file2.txt"),
		)

		state.Succeed(t, "rm", "sj://user/file1.txt").RequireFiles(t,
			ultest.File{Loc: "sj://user/file2.txt"},
			ultest.File{Loc: "/home/user/file1.txt"},
			ultest.File{Loc: "/home/user/file2.txt"},
		)
	})

	t.Run("Recursive", func(t *testing.T) {
		state := ultest.Setup(commands,
			ultest.WithFile("sj://user/files/file1.txt"),
			ultest.WithFile("sj://user/files/file2.txt"),
			ultest.WithFile("sj://user/other_file1.txt"),
			ultest.WithFile("/home/user/files/file1.txt"),
			ultest.WithFile("/home/user/files/file2.txt"),
		)

		state.Succeed(t, "rm", "sj://user/files", "-r").RequireFiles(t,
			ultest.File{Loc: "sj://user/other_file1.txt"},
			ultest.File{Loc: "/home/user/files/file1.txt"},
			ultest.File{Loc: "/home/user/files/file2.txt"},
		)
	})

	t.Run("Pending", func(t *testing.T) {
		state := ultest.Setup(commands,
			ultest.WithPendingFile("sj://user/files/file1.txt"),
			ultest.WithPendingFile("sj://user/files/file2.txt"),
			ultest.WithPendingFile("sj://user/other_file1.txt"),
		)

		state.Succeed(t, "rm", "sj://user/files/file1.txt", "--pending").RequirePending(t,
			ultest.File{Loc: "sj://user/files/file2.txt"},
			ultest.File{Loc: "sj://user/other_file1.txt"},
		)
	})

	t.Run("Pending Recursive", func(t *testing.T) {
		state := ultest.Setup(commands,
			ultest.WithPendingFile("sj://user/files/file1.txt"),
			ultest.WithPendingFile("sj://user/files/file2.txt"),
			ultest.WithPendingFile("sj://user/other_file1.txt"),
		)

		state.Succeed(t, "rm", "sj://user/files", "-r", "--pending").RequirePending(t,
			ultest.File{Loc: "sj://user/other_file1.txt"},
		)
	})
}

func TestRmLocal(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		state := ultest.Setup(commands,
			ultest.WithFile("sj://user/file1.txt"),
			ultest.WithFile("sj://user/file2.txt"),
			ultest.WithFile("/home/user/file1.txt"),
			ultest.WithFile("/home/user/file2.txt"),
		)

		state.Fail(t, "rm", "/home/user/file1.txt").RequireFiles(t,
			ultest.File{Loc: "sj://user/file1.txt"},
			ultest.File{Loc: "sj://user/file2.txt"},
			ultest.File{Loc: "/home/user/file1.txt"},
			ultest.File{Loc: "/home/user/file2.txt"},
		)
	})

	t.Run("Recursive", func(t *testing.T) {
		state := ultest.Setup(commands,
			ultest.WithFile("sj://user/files/file1.txt"),
			ultest.WithFile("sj://user/files/file2.txt"),
			ultest.WithFile("/home/user/files/file1.txt"),
			ultest.WithFile("/home/user/files/file2.txt"),
			ultest.WithFile("/home/user/other_file1.txt"),
		)

		state.Fail(t, "rm", "/home/user/files", "-r").RequireFiles(t,
			ultest.File{Loc: "sj://user/files/file1.txt"},
			ultest.File{Loc: "sj://user/files/file2.txt"},
			ultest.File{Loc: "/home/user/files/file1.txt"},
			ultest.File{Loc: "/home/user/files/file2.txt"},
			ultest.File{Loc: "/home/user/other_file1.txt"},
		)
	})
}
