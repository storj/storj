// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"

	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestLsErrors(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands)

	// empty bucket name is a parse error
	state.Fail(t, "ls", "sj:///user")
}

func TestLsRemote(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("sj://user/deep/aaa/bbb/1"),
		ultest.WithFile("sj://user/deep/aaa/bbb/2"),
		ultest.WithFile("sj://user/deep/aaa/bbb/3"),
		ultest.WithFile("sj://user/foobar"),
		ultest.WithFile("sj://user/foobar/"),
		ultest.WithFile("sj://user/foobar/1"),
		ultest.WithFile("sj://user/foobar/2"),
		ultest.WithFile("sj://user/foobar/3"),
		ultest.WithFile("sj://user/foobaz/1"),

		ultest.WithPendingFile("sj://user/invisible"),
	)

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user", "--recursive", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    24      deep/aaa/bbb/1
			OBJ     1970-01-01 00:00:02    24      deep/aaa/bbb/2
			OBJ     1970-01-01 00:00:03    24      deep/aaa/bbb/3
			OBJ     1970-01-01 00:00:04    16      foobar
			OBJ     1970-01-01 00:00:05    17      foobar/
			OBJ     1970-01-01 00:00:06    18      foobar/1
			OBJ     1970-01-01 00:00:07    18      foobar/2
			OBJ     1970-01-01 00:00:08    18      foobar/3
			OBJ     1970-01-01 00:00:09    18      foobaz/1
		`)
	})

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/fo", "--utc").RequireStdout(t, ``)
	})

	t.Run("ExactPrefix", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/foobar", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:04    16      foobar
			PRE                                    foobar/
		`)
	})

	t.Run("ExactPrefixWithSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/foobar/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:05    17
			OBJ     1970-01-01 00:00:06    18      1
			OBJ     1970-01-01 00:00:07    18      2
			OBJ     1970-01-01 00:00:08    18      3
		`)
	})

	t.Run("MultipleLayers", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/deep/").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        aaa/
		`)

		state.Succeed(t, "ls", "sj://user/deep/aaa/").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        bbb/
		`)

		state.Succeed(t, "ls", "sj://user/deep/aaa/bbb/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    24      1
			OBJ     1970-01-01 00:00:02    24      2
			OBJ     1970-01-01 00:00:03    24      3
		`)
	})
}

func TestLsJSON(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("sj://user/deep/aaa/bbb/1"),
		ultest.WithFile("sj://user/deep/aaa/bbb/2"),
		ultest.WithFile("sj://user/deep/aaa/bbb/3"),
		ultest.WithFile("sj://user/foobar"),
		ultest.WithFile("sj://user/foobar/"),
		ultest.WithFile("sj://user/foobar/1"),
		ultest.WithFile("sj://user/foobar/2"),
		ultest.WithFile("sj://user/foobar/3"),
		ultest.WithFile("sj://user/foobaz/1"),

		ultest.WithPendingFile("sj://user/invisible"),
	)

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user", "--recursive", "--utc", "--output", "json").RequireStdout(t, `
			{"kind":"OBJ","created":"1970-01-01 00:00:01","size":24,"key":"deep/aaa/bbb/1"}
			{"kind":"OBJ","created":"1970-01-01 00:00:02","size":24,"key":"deep/aaa/bbb/2"}
			{"kind":"OBJ","created":"1970-01-01 00:00:03","size":24,"key":"deep/aaa/bbb/3"}
			{"kind":"OBJ","created":"1970-01-01 00:00:04","size":16,"key":"foobar"}
			{"kind":"OBJ","created":"1970-01-01 00:00:05","size":17,"key":"foobar/"}
			{"kind":"OBJ","created":"1970-01-01 00:00:06","size":18,"key":"foobar/1"}
			{"kind":"OBJ","created":"1970-01-01 00:00:07","size":18,"key":"foobar/2"}
			{"kind":"OBJ","created":"1970-01-01 00:00:08","size":18,"key":"foobar/3"}
			{"kind":"OBJ","created":"1970-01-01 00:00:09","size":18,"key":"foobaz/1"}
		`)
	})

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/fo", "--utc", "--output", "json").RequireStdout(t, ``)
	})

	t.Run("ExactPrefix", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/foobar", "--utc", "--output", "json").RequireStdout(t, `
			{"kind":"OBJ","created":"1970-01-01 00:00:04","size":16,"key":"foobar"}
			{"kind":"PRE","key":"foobar/"}
		`)
	})

	t.Run("ShortFlag", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/foobar", "--utc", "-o", "json").RequireStdout(t, `
			{"kind":"OBJ","created":"1970-01-01 00:00:04","size":16,"key":"foobar"}
			{"kind":"PRE","key":"foobar/"}
		`)
	})

	t.Run("ExactPrefixWithSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/foobar/", "--utc", "--output", "json").RequireStdout(t, `
			{"kind":"OBJ","created":"1970-01-01 00:00:05","size":17,"key":""}
			{"kind":"OBJ","created":"1970-01-01 00:00:06","size":18,"key":"1"}
			{"kind":"OBJ","created":"1970-01-01 00:00:07","size":18,"key":"2"}
			{"kind":"OBJ","created":"1970-01-01 00:00:08","size":18,"key":"3"}
		`)
	})

	t.Run("MultipleLayers", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/deep/", "--output", "json").RequireStdout(t, `
			{"kind":"PRE","key":"aaa/"}
		`)

		state.Succeed(t, "ls", "sj://user/deep/aaa/", "--output", "json").RequireStdout(t, `
			{"kind":"PRE","key":"bbb/"}
		`)

		state.Succeed(t, "ls", "sj://user/deep/aaa/bbb/", "--utc", "--output", "json").RequireStdout(t, `
			{"kind":"OBJ","created":"1970-01-01 00:00:01","size":24,"key":"1"}
			{"kind":"OBJ","created":"1970-01-01 00:00:02","size":24,"key":"2"}
			{"kind":"OBJ","created":"1970-01-01 00:00:03","size":24,"key":"3"}
		`)
	})
}

func TestLsPending(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithPendingFile("sj://user/deep/aaa/bbb/1"),
		ultest.WithPendingFile("sj://user/deep/aaa/bbb/2"),
		ultest.WithPendingFile("sj://user/deep/aaa/bbb/3"),
		ultest.WithPendingFile("sj://user/foobar"),
		ultest.WithPendingFile("sj://user/foobar/"),
		ultest.WithPendingFile("sj://user/foobar/1"),
		ultest.WithPendingFile("sj://user/foobar/2"),
		ultest.WithPendingFile("sj://user/foobar/3"),
		ultest.WithPendingFile("sj://user/foobaz/1"),

		ultest.WithFile("sj://user/invisible"),
	)

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user", "--recursive", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    0       deep/aaa/bbb/1
			OBJ     1970-01-01 00:00:02    0       deep/aaa/bbb/2
			OBJ     1970-01-01 00:00:03    0       deep/aaa/bbb/3
			OBJ     1970-01-01 00:00:04    0       foobar
			OBJ     1970-01-01 00:00:05    0       foobar/
			OBJ     1970-01-01 00:00:06    0       foobar/1
			OBJ     1970-01-01 00:00:07    0       foobar/2
			OBJ     1970-01-01 00:00:08    0       foobar/3
			OBJ     1970-01-01 00:00:09    0       foobaz/1
		`)
	})

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/fo", "--pending", "--utc").RequireStdout(t, ``)
	})

	t.Run("ExactPrefix", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/foobar", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:04    0       foobar
			PRE                                    foobar/
		`)
	})

	t.Run("ExactPrefixWithSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/foobar/", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:05    0
			OBJ     1970-01-01 00:00:06    0       1
			OBJ     1970-01-01 00:00:07    0       2
			OBJ     1970-01-01 00:00:08    0       3
		`)
	})

	t.Run("MultipleLayers", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/deep/", "--pending").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        aaa/
		`)

		state.Succeed(t, "ls", "sj://user/deep/aaa/", "--pending").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        bbb/
		`)

		state.Succeed(t, "ls", "sj://user/deep/aaa/bbb/", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    0       1
			OBJ     1970-01-01 00:00:02    0       2
			OBJ     1970-01-01 00:00:03    0       3
		`)
	})
}

func TestLsDifficult(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
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

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user", "--recursive", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    11      /
			OBJ     1970-01-01 00:00:02    12      //
			OBJ     1970-01-01 00:00:03    13      ///
			OBJ     1970-01-01 00:00:04    23      /starts-slash
			OBJ     1970-01-01 00:00:05    20      ends-slash
			OBJ     1970-01-01 00:00:06    21      ends-slash/
			OBJ     1970-01-01 00:00:07    22      ends-slash//
			OBJ     1970-01-01 00:00:08    19      mid-slash
			OBJ     1970-01-01 00:00:09    22      mid-slash//2
			OBJ     1970-01-01 00:00:10    21      mid-slash/1
		`)
	})

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			PRE                                    /
			OBJ     1970-01-01 00:00:05    20      ends-slash
			PRE                                    ends-slash/
			OBJ     1970-01-01 00:00:08    19      mid-slash
			PRE                                    mid-slash/
		`)

		state.Succeed(t, "ls", "sj://user/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			PRE                                    /
			OBJ     1970-01-01 00:00:05    20      ends-slash
			PRE                                    ends-slash/
			OBJ     1970-01-01 00:00:08    19      mid-slash
			PRE                                    mid-slash/
		`)
	})

	t.Run("OnlySlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user//", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    11
			PRE                                    /
			OBJ     1970-01-01 00:00:04    23      starts-slash
		`)

		state.Succeed(t, "ls", "sj://user///", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:02    12
			PRE                                    /
		`)

		state.Succeed(t, "ls", "sj://user////", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:03    13
		`)
	})

	t.Run("EndsSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/ends-slash", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:05    20      ends-slash
			PRE                                    ends-slash/
		`)

		state.Succeed(t, "ls", "sj://user/ends-slash/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:06    21
			PRE                                    /
		`)

		state.Succeed(t, "ls", "sj://user/ends-slash//", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:07    22
		`)
	})

	t.Run("MidSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://user/mid-slash", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:08    19      mid-slash
			PRE                                    mid-slash/
		`)

		state.Succeed(t, "ls", "sj://user/mid-slash/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			PRE                                    /
			OBJ     1970-01-01 00:00:10    21      1
		`)

		state.Succeed(t, "ls", "sj://user/mid-slash//", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:09    22      2
		`)
	})
}

func TestLsLocal(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("/user/deep/aaa/bbb/1"),
		ultest.WithFile("/user/deep/aaa/bbb/2"),
		ultest.WithFile("/user/deep/aaa/bbb/3"),
		ultest.WithFile("/user/foobar/1"),
		ultest.WithFile("/user/foobar/2"),
		ultest.WithFile("/user/foobar/3"),
		ultest.WithFile("/user/foobaz/1"),
	)

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "/user", "--recursive", "--utc").RequireStdoutGlob(t, `
			KIND    CREATED    SIZE    KEY
			OBJ                20      /user/deep/aaa/bbb/1
			OBJ                20      /user/deep/aaa/bbb/2
			OBJ                20      /user/deep/aaa/bbb/3
			OBJ                14      /user/foobar/1
			OBJ                14      /user/foobar/2
			OBJ                14      /user/foobar/3
			OBJ                14      /user/foobaz/1
		`)
	})

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "ls", "/user/fo", "--utc").RequireStdout(t, ``)
	})

	t.Run("ExactPrefix", func(t *testing.T) {
		state.Succeed(t, "ls", "/user/foobar", "--utc").RequireStdoutGlob(t, `
			KIND    CREATED    SIZE    KEY
			OBJ                14      1
			OBJ                14      2
			OBJ                14      3
		`)
	})

	t.Run("ExactPrefixWithSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "/user/foobar/", "--utc").RequireStdoutGlob(t, `
			KIND    CREATED    SIZE    KEY
			OBJ                14      1
			OBJ                14      2
			OBJ                14      3
		`)
	})

	t.Run("MultipleLayers", func(t *testing.T) {
		state.Succeed(t, "ls", "/user/deep/").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        aaa/
		`)

		state.Succeed(t, "ls", "/user/deep/aaa/").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        bbb/
		`)

		state.Succeed(t, "ls", "/user/deep/aaa/bbb/", "--utc").RequireStdoutGlob(t, `
			KIND    CREATED    SIZE    KEY
			OBJ                20      1
			OBJ                20      2
			OBJ                20      3
		`)
	})
}

func TestLsRelative(t *testing.T) {
	state := ultest.Setup(uplinkcli.Commands,
		ultest.WithFile("deep/aaa/bbb/1"),
		ultest.WithFile("deep/aaa/bbb/2"),
		ultest.WithFile("deep/aaa/bbb/3"),
		ultest.WithFile("foobar/1"),
		ultest.WithFile("foobar/2"),
		ultest.WithFile("foobar/3"),
		ultest.WithFile("foobaz/1"),
	)

	basic := `
		KIND    CREATED    SIZE    KEY
		PRE                        deep/
		PRE                        foobar/
		PRE                        foobaz/
	`

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "ls", "", "--utc").RequireStdout(t, basic)
	})

	t.Run("BasicDot", func(t *testing.T) {
		state.Succeed(t, "ls", ".", "--utc").RequireStdout(t, basic)
	})

	t.Run("BasicDotSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "./", "--utc").RequireStdout(t, basic)
	})

	recursive := `
		KIND    CREATED    SIZE    KEY
		OBJ                14      deep/aaa/bbb/1
		OBJ                14      deep/aaa/bbb/2
		OBJ                14      deep/aaa/bbb/3
		OBJ                8       foobar/1
		OBJ                8       foobar/2
		OBJ                8       foobar/3
		OBJ                8       foobaz/1
	`

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "", "--recursive", "--utc").RequireStdoutGlob(t, recursive)
	})

	t.Run("RecursiveDot", func(t *testing.T) {
		state.Succeed(t, "ls", ".", "--recursive", "--utc").RequireStdoutGlob(t, recursive)
	})

	t.Run("RecursiveDotSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "./", "--recursive", "--utc").RequireStdoutGlob(t, recursive)
	})

}

func TestLsAllVersions(t *testing.T) {
	t.Run("Local", func(t *testing.T) {
		// Version ID should be blank for all local files.
		state := ultest.Setup(uplinkcli.Commands, ultest.WithFile("/user/foo"))
		expectedOutput{
			tabbed: `
				KIND    CREATED    SIZE    KEY    VERSION ID
				OBJ                9       foo
			`,
			json: `
				{"kind":"OBJ","created":"","size":9,"key":"foo"}
			`,
		}.check(t, state, "ls", "/user", "--all-versions", "--utc")
	})

	t.Run("Remote", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands,
			ultest.WithFile("sj://user/foo"),
			ultest.WithFile("sj://user/foo/"),
			ultest.WithFile("sj://user/foo/1"),
			ultest.WithFile("sj://user/foo/1"),
			ultest.WithFile("sj://user/foo/2"),
			ultest.WithDeleteMarker("sj://user/foo/2"),
			ultest.WithDeleteMarker("sj://user/foo/bar/baz/1"),

			ultest.WithPendingFile("sj://user/invisible"),
		)

		t.Run("Recursive", func(t *testing.T) {
			expectedOutput{
				tabbed: `
					KIND    CREATED                SIZE    KEY              VERSION ID
					OBJ     1970-01-01 00:00:01    13      foo              0000000000000000
					OBJ     1970-01-01 00:00:02    14      foo/             0000000000000000
					OBJ     1970-01-01 00:00:03    15      foo/1            0000000000000000
					OBJ     1970-01-01 00:00:04    15      foo/1            0000000000000001
					OBJ     1970-01-01 00:00:05    15      foo/2            0000000000000000
					MKR     1970-01-01 00:00:06            foo/2            0000000000000001
					MKR     1970-01-01 00:00:07            foo/bar/baz/1    0000000000000000
				`,
				json: `
					{"kind":"OBJ","created":"1970-01-01 00:00:01","size":13,"key":"foo","versionId":"0000000000000000"}
					{"kind":"OBJ","created":"1970-01-01 00:00:02","size":14,"key":"foo/","versionId":"0000000000000000"}
					{"kind":"OBJ","created":"1970-01-01 00:00:03","size":15,"key":"foo/1","versionId":"0000000000000000"}
					{"kind":"OBJ","created":"1970-01-01 00:00:04","size":15,"key":"foo/1","versionId":"0000000000000001"}
					{"kind":"OBJ","created":"1970-01-01 00:00:05","size":15,"key":"foo/2","versionId":"0000000000000000"}
					{"kind":"MKR","created":"1970-01-01 00:00:06","key":"foo/2","versionId":"0000000000000001"}
					{"kind":"MKR","created":"1970-01-01 00:00:07","key":"foo/bar/baz/1","versionId":"0000000000000000"}
				`,
			}.check(t, state, "ls", "sj://user", "--all-versions", "--recursive", "--utc")
		})

		t.Run("PartialPrefix", func(t *testing.T) {
			expectedOutput{}.check(t, state, "ls", "sj://user/fo", "--all-versions")
		})

		t.Run("ExactPrefix", func(t *testing.T) {
			expectedOutput{
				tabbed: `
					KIND    CREATED                SIZE    KEY     VERSION ID
					OBJ     1970-01-01 00:00:01    13      foo     0000000000000000
					PRE                                    foo/
				`,
				json: `
					{"kind":"OBJ","created":"1970-01-01 00:00:01","size":13,"key":"foo","versionId":"0000000000000000"}
					{"kind":"PRE","key":"foo/"}
				`,
			}.check(t, state, "ls", "sj://user/foo", "--all-versions", "--utc")
		})

		t.Run("ExactPrefixWithSlash", func(t *testing.T) {
			expectedOutput{
				tabbed: `
					KIND    CREATED                SIZE    KEY     VERSION ID
					OBJ     1970-01-01 00:00:02    14              0000000000000000
					OBJ     1970-01-01 00:00:03    15      1       0000000000000000
					OBJ     1970-01-01 00:00:04    15      1       0000000000000001
					OBJ     1970-01-01 00:00:05    15      2       0000000000000000
					MKR     1970-01-01 00:00:06            2       0000000000000001
					PRE                                    bar/
				`,
				json: `
					{"kind":"OBJ","created":"1970-01-01 00:00:02","size":14,"key":"","versionId":"0000000000000000"}
					{"kind":"OBJ","created":"1970-01-01 00:00:03","size":15,"key":"1","versionId":"0000000000000000"}
					{"kind":"OBJ","created":"1970-01-01 00:00:04","size":15,"key":"1","versionId":"0000000000000001"}
					{"kind":"OBJ","created":"1970-01-01 00:00:05","size":15,"key":"2","versionId":"0000000000000000"}
					{"kind":"MKR","created":"1970-01-01 00:00:06","key":"2","versionId":"0000000000000001"}
					{"kind":"PRE","key":"bar/"}
				`,
			}.check(t, state, "ls", "sj://user/foo/", "--all-versions", "--utc")
		})

		t.Run("MultipleLayers", func(t *testing.T) {
			expectedOutput{
				tabbed: `
					KIND    CREATED    SIZE    KEY     VERSION ID
					PRE                        baz/
				`,
				json: `
					{"kind":"PRE","key":"baz/"}
				`,
			}.check(t, state, "ls", "sj://user/foo/bar/", "--all-versions", "--utc")

			expectedOutput{
				tabbed: `
					KIND    CREATED                SIZE    KEY    VERSION ID
					MKR     1970-01-01 00:00:07            1      0000000000000000
				`,
				json: `
					{"kind":"MKR","created":"1970-01-01 00:00:07","key":"1","versionId":"0000000000000000"}
				`,
			}.check(t, state, "ls", "sj://user/foo/bar/baz/", "--all-versions", "--utc")
		})
	})
}

type expectedOutput struct {
	tabbed string
	json   string
}

func (out expectedOutput) check(t *testing.T, state ultest.State, args ...string) {
	t.Run("Tabbed Output", func(t *testing.T) {
		state.Succeed(t, args...).RequireStdout(t, out.tabbed)
	})
	t.Run("JSON Output", func(t *testing.T) {
		newArgs := append([]string(nil), args...)
		newArgs = append(newArgs, "--output", "json")
		state.Succeed(t, newArgs...).RequireStdout(t, out.json)
	})
}
