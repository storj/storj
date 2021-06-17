// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"storj.io/storj/cmd/uplinkng/ultest"
)

func TestRmRemote(t *testing.T) {
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
}

func TestRmRemoteRecursive(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("sj://user/file1.txt"),
		ultest.WithFile("sj://user/file2.txt"),
		ultest.WithFile("sj://user/other_file1.txt"),
		ultest.WithFile("/home/user/file1.txt"),
		ultest.WithFile("/home/user/file2.txt"),
	)

	state.Succeed(t, "rm", "sj://user/file", "-r").RequireFiles(t,
		ultest.File{Loc: "sj://user/other_file1.txt"},
		ultest.File{Loc: "/home/user/file1.txt"},
		ultest.File{Loc: "/home/user/file2.txt"},
	)
}

func TestRmLocal(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("sj://user/file1.txt"),
		ultest.WithFile("sj://user/file2.txt"),
		ultest.WithFile("/home/user/file1.txt"),
		ultest.WithFile("/home/user/file2.txt"),
	)

	state.Succeed(t, "rm", "/home/user/file1.txt").RequireFiles(t,
		ultest.File{Loc: "sj://user/file1.txt"},
		ultest.File{Loc: "sj://user/file2.txt"},
		ultest.File{Loc: "/home/user/file2.txt"},
	)
}

func TestRmLocalRecursive(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("sj://user/file1.txt"),
		ultest.WithFile("sj://user/file2.txt"),
		ultest.WithFile("/home/user/file1.txt"),
		ultest.WithFile("/home/user/file2.txt"),
		ultest.WithFile("/home/user/other_file1.txt"),
	)

	state.Succeed(t, "rm", "/home/user/file", "-r").RequireFiles(t,
		ultest.File{Loc: "sj://user/file1.txt"},
		ultest.File{Loc: "sj://user/file2.txt"},
		ultest.File{Loc: "/home/user/other_file1.txt"},
	)
}
