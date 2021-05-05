// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
)

func TestLsErrors(t *testing.T) {
	state := Setup(t)

	// empty bucket name is a parse error
	state.Fail(t, "ls", "sj:///jeff")
}

func TestLsRemote(t *testing.T) {
	state := Setup(t,
		WithFile("sj://jeff/deep/aaa/bbb/1"),
		WithFile("sj://jeff/deep/aaa/bbb/2"),
		WithFile("sj://jeff/deep/aaa/bbb/3"),
		WithFile("sj://jeff/foobar"),
		WithFile("sj://jeff/foobar/"),
		WithFile("sj://jeff/foobar/1"),
		WithFile("sj://jeff/foobar/2"),
		WithFile("sj://jeff/foobar/3"),
		WithFile("sj://jeff/foobaz/1"),

		WithPendingFile("sj://jeff/invisible"),
	)

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff", "--recursive", "--utc").RequireStdout(t, `
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
		state.Succeed(t, "ls", "sj://jeff/fo", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:04    0       foobar
			PRE                                    foobar/
			PRE                                    foobaz/
		`)
	})

	t.Run("ExactPrefix", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/foobar", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:04    0       foobar
			PRE                                    foobar/
		`)
	})

	t.Run("ExactPrefixWithSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/foobar/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:05    0
			OBJ     1970-01-01 00:00:06    0       1
			OBJ     1970-01-01 00:00:07    0       2
			OBJ     1970-01-01 00:00:08    0       3
		`)
	})

	t.Run("MultipleLayers", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/deep/").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        aaa/
		`)

		state.Succeed(t, "ls", "sj://jeff/deep/aaa/").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        bbb/
		`)

		state.Succeed(t, "ls", "sj://jeff/deep/aaa/bbb/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    0       1
			OBJ     1970-01-01 00:00:02    0       2
			OBJ     1970-01-01 00:00:03    0       3
		`)
	})
}

func TestLsPending(t *testing.T) {
	state := Setup(t,
		WithPendingFile("sj://jeff/deep/aaa/bbb/1"),
		WithPendingFile("sj://jeff/deep/aaa/bbb/2"),
		WithPendingFile("sj://jeff/deep/aaa/bbb/3"),
		WithPendingFile("sj://jeff/foobar"),
		WithPendingFile("sj://jeff/foobar/"),
		WithPendingFile("sj://jeff/foobar/1"),
		WithPendingFile("sj://jeff/foobar/2"),
		WithPendingFile("sj://jeff/foobar/3"),
		WithPendingFile("sj://jeff/foobaz/1"),

		WithFile("sj://jeff/invisible"),
	)

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff", "--recursive", "--pending", "--utc").RequireStdout(t, `
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
		state.Succeed(t, "ls", "sj://jeff/fo", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:04    0       foobar
			PRE                                    foobar/
			PRE                                    foobaz/
		`)
	})

	t.Run("ExactPrefix", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/foobar", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:04    0       foobar
			PRE                                    foobar/
		`)
	})

	t.Run("ExactPrefixWithSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/foobar/", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:05    0
			OBJ     1970-01-01 00:00:06    0       1
			OBJ     1970-01-01 00:00:07    0       2
			OBJ     1970-01-01 00:00:08    0       3
		`)
	})

	t.Run("MultipleLayers", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/deep/", "--pending").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        aaa/
		`)

		state.Succeed(t, "ls", "sj://jeff/deep/aaa/", "--pending").RequireStdout(t, `
			KIND    CREATED    SIZE    KEY
			PRE                        bbb/
		`)

		state.Succeed(t, "ls", "sj://jeff/deep/aaa/bbb/", "--pending", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    0       1
			OBJ     1970-01-01 00:00:02    0       2
			OBJ     1970-01-01 00:00:03    0       3
		`)
	})
}

func TestLsDifficult(t *testing.T) {
	state := Setup(t,
		WithFile("sj://jeff//"),
		WithFile("sj://jeff///"),
		WithFile("sj://jeff////"),

		WithFile("sj://jeff//starts-slash"),

		WithFile("sj://jeff/ends-slash"),
		WithFile("sj://jeff/ends-slash/"),
		WithFile("sj://jeff/ends-slash//"),

		WithFile("sj://jeff/mid-slash"),
		WithFile("sj://jeff/mid-slash//2"),
		WithFile("sj://jeff/mid-slash/1"),
	)

	t.Run("Recursive", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff", "--recursive", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    0       /
			OBJ     1970-01-01 00:00:02    0       //
			OBJ     1970-01-01 00:00:03    0       ///
			OBJ     1970-01-01 00:00:04    0       /starts-slash
			OBJ     1970-01-01 00:00:05    0       ends-slash
			OBJ     1970-01-01 00:00:06    0       ends-slash/
			OBJ     1970-01-01 00:00:07    0       ends-slash//
			OBJ     1970-01-01 00:00:08    0       mid-slash
			OBJ     1970-01-01 00:00:09    0       mid-slash//2
			OBJ     1970-01-01 00:00:10    0       mid-slash/1
		`)
	})

	t.Run("Basic", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			PRE                                    /
			OBJ     1970-01-01 00:00:05    0       ends-slash
			PRE                                    ends-slash/
			OBJ     1970-01-01 00:00:08    0       mid-slash
			PRE                                    mid-slash/
		`)

		state.Succeed(t, "ls", "sj://jeff/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			PRE                                    /
			OBJ     1970-01-01 00:00:05    0       ends-slash
			PRE                                    ends-slash/
			OBJ     1970-01-01 00:00:08    0       mid-slash
			PRE                                    mid-slash/
		`)
	})

	t.Run("OnlySlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff//", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:01    0
			PRE                                    /
			OBJ     1970-01-01 00:00:04    0       starts-slash
		`)

		state.Succeed(t, "ls", "sj://jeff///", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:02    0
			PRE                                    /
		`)

		state.Succeed(t, "ls", "sj://jeff////", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:03    0
		`)
	})

	t.Run("EndsSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/ends-slash", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:05    0       ends-slash
			PRE                                    ends-slash/
		`)

		state.Succeed(t, "ls", "sj://jeff/ends-slash/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:06    0
			PRE                                    /
		`)

		state.Succeed(t, "ls", "sj://jeff/ends-slash//", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:07    0
		`)
	})

	t.Run("MidSlash", func(t *testing.T) {
		state.Succeed(t, "ls", "sj://jeff/mid-slash", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:08    0       mid-slash
			PRE                                    mid-slash/
		`)

		state.Succeed(t, "ls", "sj://jeff/mid-slash/", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			PRE                                    /
			OBJ     1970-01-01 00:00:10    0       1
		`)

		state.Succeed(t, "ls", "sj://jeff/mid-slash//", "--utc").RequireStdout(t, `
			KIND    CREATED                SIZE    KEY
			OBJ     1970-01-01 00:00:09    0       2
		`)
	})
}
