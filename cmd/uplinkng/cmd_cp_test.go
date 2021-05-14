// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"storj.io/storj/cmd/uplinkng/ultest"
)

func TestCpDownload(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("sj://user/file1.txt", "remote"),
	)

	state.Succeed(t, "cp", "sj://user/file1.txt", "/home/user/file1.txt").RequireFiles(t,
		ultest.File{Loc: "/home/user/file1.txt", Contents: "remote"},
		ultest.File{Loc: "sj://user/file1.txt", Contents: "remote"},
	)
}

func TestCpDownloadOverwrite(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("/home/user/file1.txt", "local"),
		ultest.WithFile("sj://user/file1.txt", "remote"),
	)

	state.Succeed(t, "cp", "sj://user/file1.txt", "/home/user/file1.txt").RequireFiles(t,
		ultest.File{Loc: "/home/user/file1.txt", Contents: "remote"},
		ultest.File{Loc: "sj://user/file1.txt", Contents: "remote"},
	)
}

func TestCpUpload(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("/home/user/file1.txt", "local"),
		ultest.WithBucket("user"),
	)

	state.Succeed(t, "cp", "/home/user/file1.txt", "sj://user/file1.txt").RequireFiles(t,
		ultest.File{Loc: "/home/user/file1.txt", Contents: "local"},
		ultest.File{Loc: "sj://user/file1.txt", Contents: "local"},
	)
}

func TestCpUploadOverwrite(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("/home/user/file1.txt", "local"),
		ultest.WithFile("sj://user/file1.txt", "remote"),
	)

	state.Succeed(t, "cp", "/home/user/file1.txt", "sj://user/file1.txt").RequireFiles(t,
		ultest.File{Loc: "/home/user/file1.txt", Contents: "local"},
		ultest.File{Loc: "sj://user/file1.txt", Contents: "local"},
	)
}

func TestCpRecursiveDownload(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("sj://user/file1.txt", "data1"),
		ultest.WithFile("sj://user/folder1/file2.txt", "data2"),
		ultest.WithFile("sj://user/folder1/file3.txt", "data3"),
		ultest.WithFile("sj://user/folder2/folder3/file4.txt", "data4"),
		ultest.WithFile("sj://user/folder2/folder3/file5.txt", "data5"),
	)

	state.Succeed(t, "cp", "sj://user", "/home/user/dest", "--recursive").RequireFiles(t,
		ultest.File{Loc: "sj://user/file1.txt", Contents: "data1"},
		ultest.File{Loc: "sj://user/folder1/file2.txt", Contents: "data2"},
		ultest.File{Loc: "sj://user/folder1/file3.txt", Contents: "data3"},
		ultest.File{Loc: "sj://user/folder2/folder3/file4.txt", Contents: "data4"},
		ultest.File{Loc: "sj://user/folder2/folder3/file5.txt", Contents: "data5"},

		ultest.File{Loc: "/home/user/dest/file1.txt", Contents: "data1"},
		ultest.File{Loc: "/home/user/dest/folder1/file2.txt", Contents: "data2"},
		ultest.File{Loc: "/home/user/dest/folder1/file3.txt", Contents: "data3"},
		ultest.File{Loc: "/home/user/dest/folder2/folder3/file4.txt", Contents: "data4"},
		ultest.File{Loc: "/home/user/dest/folder2/folder3/file5.txt", Contents: "data5"},
	)

	state.Succeed(t, "cp", "sj://user/fo", "/home/user/dest", "--recursive").RequireFiles(t,
		ultest.File{Loc: "sj://user/file1.txt", Contents: "data1"},
		ultest.File{Loc: "sj://user/folder1/file2.txt", Contents: "data2"},
		ultest.File{Loc: "sj://user/folder1/file3.txt", Contents: "data3"},
		ultest.File{Loc: "sj://user/folder2/folder3/file4.txt", Contents: "data4"},
		ultest.File{Loc: "sj://user/folder2/folder3/file5.txt", Contents: "data5"},

		ultest.File{Loc: "/home/user/dest/folder1/file2.txt", Contents: "data2"},
		ultest.File{Loc: "/home/user/dest/folder1/file3.txt", Contents: "data3"},
		ultest.File{Loc: "/home/user/dest/folder2/folder3/file4.txt", Contents: "data4"},
		ultest.File{Loc: "/home/user/dest/folder2/folder3/file5.txt", Contents: "data5"},
	)
}

func TestCpRecursiveDifficult(t *testing.T) {
	state := ultest.Setup(commands,
		ultest.WithFile("sj://user/dot-dot/../foo"),
		ultest.WithFile("sj://user/dot-dot/../../foo"),

		ultest.WithFile("sj://user//"),
		ultest.WithFile("sj://user///"),
		ultest.WithFile("sj://user////"),

		ultest.WithFile("sj://user//starts-slash"),

		ultest.WithFile("sj://user/ends-slash"),
		ultest.WithFile("sj://user/ends-slash/"),
		ultest.WithFile("sj://user/ends-slash//"),

		ultest.WithFile("sj://user/mid-slash"),
		ultest.WithFile("sj://user/mid-slash//2"),
		ultest.WithFile("sj://user/mid-slash/1"),
	)

	// TODO(jeff): these tests. oops.
	_ = state
}
