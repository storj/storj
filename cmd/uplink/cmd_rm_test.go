// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestRmErrors(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands)

	t.Run("Version ID with Pending", func(t *testing.T) {
		state.Fail(t, "rm", "sj://user/file.txt", "--version-id", "0000000000000001", "--pending")
	})

	t.Run("Version ID with Recursive", func(t *testing.T) {
		state.Fail(t, "rm", "sj://user/file.txt", "--version-id", "0000000000000001", "--recursive")
	})

	t.Run("Version ID with Local Location", func(t *testing.T) {
		state.Fail(t, "rm", "/user/file.txt", "--version-id", "0000000000000001")
	})
}

func TestRmRemote(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
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
		state := ultest.Setup(uplinkcli.Commands,
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
		state := ultest.Setup(uplinkcli.Commands,
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
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithPendingFile("sj://user/files/file1.txt"),
			ultest.WithPendingFile("sj://user/files/file2.txt"),
			ultest.WithPendingFile("sj://user/other_file1.txt"),
		)

		state.Succeed(t, "rm", "sj://user/files", "-r", "--pending").RequirePending(t,
			ultest.File{Loc: "sj://user/other_file1.txt"},
		)
	})

	t.Run("Version ID", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user/file.txt"),
			ultest.WithFile("sj://user/file.txt"),
			ultest.WithFile("sj://user/file.txt"),
		)

		state.Succeed(t, "rm", "sj://user/file.txt", "--version-id", "0000000000000001").RequireFiles(t,
			ultest.File{Loc: "sj://user/file.txt", Version: 0},
			ultest.File{Loc: "sj://user/file.txt", Version: 2},
		)
	})

	t.Run("Version ID with Governance Locked File", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands, ultest.WithGovernanceLockedFile("sj://user/file.txt"))

		state.Fail(t, "rm", "sj://user/file.txt", "--version-id", "0000000000000000").RequireFiles(t,
			ultest.File{Loc: "sj://user/file.txt", Version: 0},
		)

		state.Succeed(t, "rm", "sj://user/file.txt", "--version-id", "0000000000000000", "--bypass-governance-retention").RequireFiles(t)
	})
}

func TestRmLocal(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
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
		state := ultest.Setup(uplinkcli.Commands,
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
