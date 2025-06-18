// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestCpDownload(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("/home/user/file1.txt", "local"),
		ultest.WithFile("sj://user/file1.txt", "remote"),
	)

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/file1.txt", "/home/user/file2.txt").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local"},
			ultest.File{Loc: "/home/user/file2.txt", Contents: "remote"},
		)
	})

	t.Run("Overwrite", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/file1.txt", "/home/user/file1.txt").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "remote"},
		)
	})

	t.Run("Relative", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/file1.txt", "").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local"},
			ultest.File{Loc: "file1.txt", Contents: "remote"},
		)

		state.Succeed(t, "cp", "sj://user/file1.txt", ".").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local"},
			ultest.File{Loc: "file1.txt", Contents: "remote"},
		)

		state.Succeed(t, "cp", "sj://user/file1.txt", "./").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local"},
			ultest.File{Loc: "file1.txt", Contents: "remote"},
		)
	})

	t.Run("Recursive", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user/file1.txt", "data1"),
			ultest.WithFile("sj://user/folder1/file2.txt", "data2"),
			ultest.WithFile("sj://user/folder1/file3.txt", "data3"),
			ultest.WithFile("sj://user/folder1/folder2/file4.txt", "data4"),
			ultest.WithFile("sj://user/folder1/folder2/file5.txt", "data5"),
		)

		state.Succeed(t, "cp", "sj://user", "/home/user/dest", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/dest/file1.txt", Contents: "data1"},
			ultest.File{Loc: "/home/user/dest/folder1/file2.txt", Contents: "data2"},
			ultest.File{Loc: "/home/user/dest/folder1/file3.txt", Contents: "data3"},
			ultest.File{Loc: "/home/user/dest/folder1/folder2/file4.txt", Contents: "data4"},
			ultest.File{Loc: "/home/user/dest/folder1/folder2/file5.txt", Contents: "data5"},
		)

		state.Succeed(t, "cp", "sj://user/folder1", "/home/user/dest", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/dest/folder1/file2.txt", Contents: "data2"},
			ultest.File{Loc: "/home/user/dest/folder1/file3.txt", Contents: "data3"},
			ultest.File{Loc: "/home/user/dest/folder1/folder2/file4.txt", Contents: "data4"},
			ultest.File{Loc: "/home/user/dest/folder1/folder2/file5.txt", Contents: "data5"},
		)

		state.Succeed(t, "cp", "sj://user/fo", "/home/user/dest", "--recursive").RequireLocalFiles(t)
	})

	t.Run("Range", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user/file-for-byte-range", "abcdefghijklmnopqrstuvwxyz"),
		)

		// multiple byte ranges not supported
		state.Fail(t, "cp", "sj://user/file-for-byte-range", "/home/user/dest/file-for-byte-range", "--range", "bytes=0-1,2-3").RequireFailure(t).RequireLocalFiles(t)

		state.Succeed(t, "cp", "sj://user/file-for-byte-range", "/home/user/dest/file-for-byte-range", "--range", "bytes=0-2").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/dest/file-for-byte-range", Contents: "abc"},
		)

		state.Succeed(t, "cp", "sj://user/file-for-byte-range", "/home/user/dest/file-for-byte-range", "--range", "bytes=-1").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/dest/file-for-byte-range", Contents: "z"},
		)

		// invalid range
		state.Fail(t, "cp", "sj://user/file-for-byte-range", "/home/user/dest/file-for-byte-range", "--range", "bytes=0,-1").RequireFailure(t).RequireLocalFiles(t)
	})
}

func TestCpUpload(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("/home/user/file1.txt", "local"),
		ultest.WithFile("sj://user/file1.txt", "remote"),
	)

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "cp", "/home/user/file1.txt", "sj://user/file2.txt").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://user/file2.txt", Contents: "local"},
		)
	})

	t.Run("Overwrite", func(t *testing.T) {
		state.Succeed(t, "cp", "/home/user/file1.txt", "sj://user/file1.txt").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "local"},
		)
	})

	t.Run("Expires", func(t *testing.T) {
		state.Succeed(t, "cp", "--expires", "+4h", "/home/user/file1.txt", "sj://user/file1.txt").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "local"},
		)

		state.Succeed(t, "cp", "--expires", "-4h", "/home/user/file1.txt", "sj://user/file1.txt").RequireRemoteFiles(t)
	})

	t.Run("EdgeCases", func(t *testing.T) {
		state.Succeed(t, "cp", "/home/user/file1.txt", "sj://user").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "local"},
		)

		state.Succeed(t, "cp", "/home/user/file1.txt", "sj://user/foo").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://user/foo", Contents: "local"},
		)

		state.Succeed(t, "cp", "/home/user/file1.txt", "sj://user/foo/").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://user/foo/file1.txt", Contents: "local"},
		)
	})

	t.Run("Recursive", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithBucket("user"),
			ultest.WithFile("/home/user/file1.txt", "data1"),
			ultest.WithFile("/home/user/file2.txt", "data2"),
			ultest.WithFile("/home/user/file3.txt", "data3"),
		)

		state.Succeed(t, "cp", "/home/user", "sj://user/folder", "--recursive").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/folder/file1.txt", Contents: "data1"},
			ultest.File{Loc: "sj://user/folder/file2.txt", Contents: "data2"},
			ultest.File{Loc: "sj://user/folder/file3.txt", Contents: "data3"},
		)

		state.Succeed(t, "cp", "/home/user/fi", "sj://user/folder", "--recursive").RequireRemoteFiles(t)
	})

	t.Run("Metadata", func(t *testing.T) {
		state.Succeed(t, "cp", "--metadata", "{\"key\":\"value\"}", "/home/user/file1.txt", "sj://user/file_with_metadata.txt").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote"},
			ultest.File{Loc: "sj://user/file_with_metadata.txt", Contents: "local", Metadata: map[string]string{
				"key": "value",
			}},
		)
	})
}

func TestCpRecursiveDifficult(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user/dot-dot/../../../../../foo"),
			ultest.WithFile("sj://user/dot-dot/../../foo"),
			ultest.WithFile("sj://user/dot-dot/../foo"),
			ultest.WithFile("sj://user//starts-slash"),
			ultest.WithFile("sj://user/ends-slash"),
			ultest.WithFile("sj://user/ends-slash/"),
			ultest.WithFile("sj://user/ends-slash//"),
			ultest.WithFile("sj://user/mid-slash//file"),
		)

		state.Succeed(t, "cp", "sj://user", "some/deep/folder", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "some/deep/folder/foo", Contents: "sj://user/dot-dot/../foo"},
			ultest.File{Loc: "some/deep/folder/starts-slash", Contents: "sj://user//starts-slash"},
			ultest.File{Loc: "some/deep/folder/ends-slash", Contents: "sj://user/ends-slash//"},
			ultest.File{Loc: "some/deep/folder/mid-slash/file", Contents: "sj://user/mid-slash//file"},
		)
	})

	t.Run("DirectoryConflict", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user/filedir"),
			ultest.WithFile("sj://user/filedir/file"),
		)

		state.Fail(t, "cp", "sj://user", "root", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "root/filedir", Contents: "sj://user/filedir"},
		)
	})

	t.Run("EmptyIntoEmpty", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user//"),
		)

		state.Fail(t, "cp", "sj://user", "", "--recursive")
	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user/filedir"),
			ultest.WithFile("/home/user/filedir/file"),
		)

		state.Fail(t, "cp", "sj://user", "/home/user", "--recursive")
	})
}

func TestCpRemoteToRemote(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("sj://b1/dot-dot/../../../../../foo", "data1"),
		ultest.WithFile("sj://b1/dot-dot/../../foo", "data2"),
		ultest.WithFile("sj://b1/dot-dot/../foo", "data3"),
		ultest.WithFile("sj://b1//starts-slash", "data4"),
		ultest.WithFile("sj://b1/ends-slash", "data5"),
		ultest.WithFile("sj://b1/ends-slash/", "data6"),
		ultest.WithFile("sj://b1/ends-slash//", "data7"),
		ultest.WithFile("sj://b1/mid-slash//file", "data8"),
		ultest.WithBucket("b2"),
	)

	t.Run("BucketToBucket", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://b1", "sj://b2", "--recursive").RequireFiles(t,
			ultest.File{Loc: "sj://b1/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b1/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b1/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b1//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b1/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b1/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b1/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b1/mid-slash//file", Contents: "data8"},

			ultest.File{Loc: "sj://b2/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b2/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b2/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b2//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b2/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b2/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b2/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b2/mid-slash//file", Contents: "data8"},
		)
	})

	t.Run("BucketToPrefix", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://b1", "sj://b2/pre", "--recursive").RequireFiles(t,
			ultest.File{Loc: "sj://b1/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b1/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b1/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b1//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b1/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b1/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b1/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b1/mid-slash//file", Contents: "data8"},

			ultest.File{Loc: "sj://b2/pre/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b2/pre/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b2/pre/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b2/pre//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b2/pre/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b2/pre/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b2/pre/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b2/pre/mid-slash//file", Contents: "data8"},
		)
	})

	t.Run("PrefixToBucket", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://b1/dot-dot", "sj://b2", "--recursive").RequireFiles(t,
			ultest.File{Loc: "sj://b1/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b1/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b1/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b1//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b1/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b1/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b1/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b1/mid-slash//file", Contents: "data8"},

			ultest.File{Loc: "sj://b2/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b2/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b2/dot-dot/../foo", Contents: "data3"},
		)
	})

	t.Run("PrefixToPrefix", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://b1/dot-dot", "sj://b2/pre", "--recursive").RequireFiles(t,
			ultest.File{Loc: "sj://b1/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b1/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b1/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b1//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b1/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b1/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b1/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b1/mid-slash//file", Contents: "data8"},

			ultest.File{Loc: "sj://b2/pre/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b2/pre/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b2/pre/dot-dot/../foo", Contents: "data3"},
		)
	})

	t.Run("ObjectToObject", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://b1/ends-slash", "sj://b1/ends-slash-copy").RequireFiles(t,
			ultest.File{Loc: "sj://b1/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b1/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b1/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b1//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b1/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b1/ends-slash-copy", Contents: "data5"},
			ultest.File{Loc: "sj://b1/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b1/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b1/mid-slash//file", Contents: "data8"},
		)
	})

	t.Run("Expires", func(t *testing.T) {
		state.Fail(t, "cp", "--expires", "+4h", "sj://b1/ends-slash", "sj://b1/ends-slash-copy").RequireFiles(t,
			ultest.File{Loc: "sj://b1/dot-dot/../../../../../foo", Contents: "data1"},
			ultest.File{Loc: "sj://b1/dot-dot/../../foo", Contents: "data2"},
			ultest.File{Loc: "sj://b1/dot-dot/../foo", Contents: "data3"},
			ultest.File{Loc: "sj://b1//starts-slash", Contents: "data4"},
			ultest.File{Loc: "sj://b1/ends-slash", Contents: "data5"},
			ultest.File{Loc: "sj://b1/ends-slash/", Contents: "data6"},
			ultest.File{Loc: "sj://b1/ends-slash//", Contents: "data7"},
			ultest.File{Loc: "sj://b1/mid-slash//file", Contents: "data8"},
		)
	})
}

func TestCpLocalToLocal(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("/home/user1/folder1/file1.txt", "data1"),
		ultest.WithFile("/home/user1/folder1/file2.txt", "data2"),
		ultest.WithFile("/home/user1/folder2/file3.txt", "data3"),
		ultest.WithFile("/home/user1/folder2/folder3/file4.txt", "data4"),
		ultest.WithFile("/home/user1/folder2/folder3/file5.txt", "data4"),
	)

	t.Run("FolderToFolder", func(t *testing.T) {
		state.Fail(t, "cp", "/home/user1", "/home/user2", "--recursive").RequireFiles(t,
			ultest.File{Loc: "/home/user1/folder1/file1.txt", Contents: "data1"},
			ultest.File{Loc: "/home/user1/folder1/file2.txt", Contents: "data2"},
			ultest.File{Loc: "/home/user1/folder2/file3.txt", Contents: "data3"},
			ultest.File{Loc: "/home/user1/folder2/folder3/file4.txt", Contents: "data4"},
			ultest.File{Loc: "/home/user1/folder2/folder3/file5.txt", Contents: "data4"},
		)
	})

	t.Run("FolderToEmpty", func(t *testing.T) {
		state.Fail(t, "cp", "/home/user1", "", "--recursive").RequireFiles(t,
			ultest.File{Loc: "/home/user1/folder1/file1.txt", Contents: "data1"},
			ultest.File{Loc: "/home/user1/folder1/file2.txt", Contents: "data2"},
			ultest.File{Loc: "/home/user1/folder2/file3.txt", Contents: "data3"},
			ultest.File{Loc: "/home/user1/folder2/folder3/file4.txt", Contents: "data4"},
			ultest.File{Loc: "/home/user1/folder2/folder3/file5.txt", Contents: "data4"},
		)
	})

	t.Run("RootToFolder", func(t *testing.T) {
		state.Fail(t, "cp", "/", "/pre", "--recursive").RequireFiles(t,
			ultest.File{Loc: "/home/user1/folder1/file1.txt", Contents: "data1"},
			ultest.File{Loc: "/home/user1/folder1/file2.txt", Contents: "data2"},
			ultest.File{Loc: "/home/user1/folder2/file3.txt", Contents: "data3"},
			ultest.File{Loc: "/home/user1/folder2/folder3/file4.txt", Contents: "data4"},
			ultest.File{Loc: "/home/user1/folder2/folder3/file5.txt", Contents: "data4"},
		)
	})
}

func TestCpTrailingSlashes(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("sj://user/foo/"),
	)

	t.Run("Single", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/foo/", "pre").RequireLocalFiles(t,
			ultest.File{Loc: "pre", Contents: "sj://user/foo/"},
		)
	})

	t.Run("SingleIntoFolder", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/foo/", "pre/").RequireLocalFiles(t,
			ultest.File{Loc: "pre/foo", Contents: "sj://user/foo/"},
		)
	})

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/", "pre", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "pre/foo", Contents: "sj://user/foo/"},
		)
	})

	t.Run("RecursiveIntoFolder", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/", "pre/", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "pre/foo", Contents: "sj://user/foo/"},
		)
	})

	t.Run("RecursiveOnFile", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/foo/", "pre", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "pre", Contents: "sj://user/foo/"},
		)
	})

	t.Run("RecursiveOnFileIntoFolder", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/foo/", "pre/", "--recursive").RequireLocalFiles(t,
			ultest.File{Loc: "pre", Contents: "sj://user/foo/"},
		)
	})
}

func TestCpStandard(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("sj://user/foo"),
		ultest.WithFile("/home/user/foo"),
		ultest.WithStdin("-"),
	)

	t.Run("StdinToRemote", func(t *testing.T) {
		state.Succeed(t, "cp", "-", "sj://user/bar").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/foo"},
			ultest.File{Loc: "sj://user/bar", Contents: "-"},
		)
	})

	t.Run("StdinToRemoteTrailing", func(t *testing.T) {
		state.Succeed(t, "cp", "-", "sj://user/bar/").RequireRemoteFiles(t,
			ultest.File{Loc: "sj://user/foo"},
			ultest.File{Loc: "sj://user/bar/", Contents: "-"},
		)
	})

	t.Run("StdinToLocal", func(t *testing.T) {
		state.Fail(t, "cp", "-", "/home/user/bar").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/foo"},
		)
	})

	t.Run("StdinToLocalTrailing", func(t *testing.T) {
		state.Fail(t, "cp", "-", "/home/user/bar/").RequireLocalFiles(t,
			ultest.File{Loc: "/home/user/foo"},
		)
	})

	t.Run("RemoteToStdout", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/foo", "-").RequireFiles(t,
			ultest.File{Loc: "sj://user/foo"},
			ultest.File{Loc: "/home/user/foo"},
		)
	})

	t.Run("LocalToStdout", func(t *testing.T) {
		state.Fail(t, "cp", "/home/user/foo", "-").RequireFiles(t,
			ultest.File{Loc: "sj://user/foo"},
			ultest.File{Loc: "/home/user/foo"},
		)
	})
}

func TestCpInputValidation(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands)

	state.Fail(t, "cp", "/home/user/file1.txt", "sj://testbucket/", "--parallelism-chunk-size", "-1")
}

func TestCpMultipleSourcePaths(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("/home/user/file1.txt", "local1"),
		ultest.WithFile("/home/user/file2.txt", "local2"),
		ultest.WithFile("sj://user/file1.txt", "remote1"),
		ultest.WithFile("sj://user/file2.txt", "remote2"),
	)

	t.Run("LocalToRemoteOverwrite", func(t *testing.T) {
		state.Succeed(t, "cp", "/home/user/file1.txt", "/home/user/file2.txt", "sj://user").RequireFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local1"},
			ultest.File{Loc: "/home/user/file2.txt", Contents: "local2"},

			ultest.File{Loc: "sj://user/file1.txt", Contents: "local1"},
			ultest.File{Loc: "sj://user/file2.txt", Contents: "local2"},
		)
	})

	t.Run("LocalToRemoteDirectory", func(t *testing.T) {
		state.Succeed(t, "cp", "/home/user/file1.txt", "/home/user/file2.txt", "sj://user/new").RequireFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local1"},
			ultest.File{Loc: "/home/user/file2.txt", Contents: "local2"},
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "sj://user/file2.txt", Contents: "remote2"},

			ultest.File{Loc: "sj://user/new/file1.txt", Contents: "local1"},
			ultest.File{Loc: "sj://user/new/file2.txt", Contents: "local2"},
		)
	})

	t.Run("RemoteToLocalOverwrite", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/file1.txt", "sj://user/file2.txt", "/home/user").RequireFiles(t,
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "sj://user/file2.txt", Contents: "remote2"},

			ultest.File{Loc: "/home/user/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "/home/user/file2.txt", Contents: "remote2"},
		)
	})

	t.Run("RemoteToLocalDirectory", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/file1.txt", "sj://user/file2.txt", "/home/user/new").RequireFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local1"},
			ultest.File{Loc: "/home/user/file2.txt", Contents: "local2"},
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "sj://user/file2.txt", Contents: "remote2"},

			ultest.File{Loc: "/home/user/new/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "/home/user/new/file2.txt", Contents: "remote2"},
		)
	})

	t.Run("MixedToRemoteDirectory", func(t *testing.T) {
		state.Succeed(t, "cp", "sj://user/file1.txt", "/home/user/file2.txt", "sj://user/new").RequireFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local1"},
			ultest.File{Loc: "/home/user/file2.txt", Contents: "local2"},
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "sj://user/file2.txt", Contents: "remote2"},

			ultest.File{Loc: "sj://user/new/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "sj://user/new/file2.txt", Contents: "local2"},
		)
	})

	t.Run("MixedToLocalPartiallyFails", func(t *testing.T) {
		state.Fail(t, "cp", "sj://user/file1.txt", "/home/user/file2.txt", "/home/user/new").RequireFiles(t,
			ultest.File{Loc: "/home/user/file1.txt", Contents: "local1"},
			ultest.File{Loc: "/home/user/file2.txt", Contents: "local2"},
			ultest.File{Loc: "sj://user/file1.txt", Contents: "remote1"},
			ultest.File{Loc: "sj://user/file2.txt", Contents: "remote2"},

			ultest.File{Loc: "/home/user/new/file1.txt", Contents: "remote1"},
		)
	})
}
