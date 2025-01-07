// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/metabase"
)

func TestListObjects_startCursor(t *testing.T) {
	// There are few behaviors that need to be mentioned multiple times:
	//
	// latest version double check -- we need to start listing AllVersions = 0
	// from the first possible versions, because we may need to skip the whole
	// entry when the cursor is just past it.
	// PS: this will be unnecessary when we add isLatest column to the database.
	//
	// skip outside for non-recursive -- we started iterating from an entry
	// that is IsPrefix=true, so we need to skip all the entries.

	// empty cursor starting behaviour
	for _, allVersions := range []bool{false, true} {
		for _, recursive := range []bool{false, true} {
			for _, pending := range []bool{false, true} {
				opts := metabase.ListObjects{
					AllVersions: allVersions,
					Recursive:   recursive,
					Pending:     pending,
					Cursor:      metabase.ListObjectsCursor{},
				}

				switch {
				case !allVersions:
					// latest version double check, optional
					assert.Equal(t, metabase.ListObjectsCursor{
						Key:     "",
						Version: opts.FirstVersion(),
					}, opts.StartCursor(), opts)
				default:
					assert.Equal(t, metabase.ListObjectsCursor{
						Key:     "",
						Version: 0,
					}, opts.StartCursor(), opts)
				}
			}
		}
	}

	// plain simple cursor
	for _, allVersions := range []bool{false, true} {
		for _, recursive := range []bool{false, true} {
			for _, pending := range []bool{false, true} {
				opts := metabase.ListObjects{
					AllVersions: allVersions,
					Recursive:   recursive,
					Pending:     pending,
					Cursor: metabase.ListObjectsCursor{
						Key:     "a",
						Version: 100,
					},
				}

				switch {
				case !allVersions:
					// latest version double check
					assert.Equal(t, metabase.ListObjectsCursor{
						Key:     "a",
						Version: opts.FirstVersion(),
					}, opts.StartCursor(), opts)

				case allVersions:
					assert.Equal(t, metabase.ListObjectsCursor{
						Key:     "a",
						Version: 100,
					}, opts.StartCursor(), opts)

				default:
					panic("unhandled scenario")
				}
			}
		}
	}

	// cursor with nesting
	for _, prefix := range []metabase.ObjectKey{"", "x/", "x/x/", "/", "//"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := metabase.ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: metabase.ListObjectsCursor{
							Key:     prefix + "a/a",
							Version: 100,
						},
						Prefix: prefix,
					}

					switch {
					case recursive && allVersions:
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     prefix + "a/a",
							Version: 100,
						}, opts.StartCursor(), opts)

					case recursive && !allVersions:
						// latest version double check
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     prefix + "a/a",
							Version: opts.FirstVersion(),
						}, opts.StartCursor(), opts)

					case !recursive:
						// skip outside for non-recursive
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     prefix + "a" + metabase.DelimiterNext,
							Version: opts.FirstVersion(),
						}, opts.StartCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}

	// cursor inside a prefix
	for _, prefix := range []metabase.ObjectKey{"a/", "a/a/", "/", "//"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := metabase.ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: metabase.ListObjectsCursor{
							Key:     prefix + "a",
							Version: 100,
						},
						Prefix: prefix,
					}

					switch {
					case !allVersions:
						// latest version double check
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     prefix + "a",
							Version: opts.FirstVersion(),
						}, opts.StartCursor(), opts)

					case allVersions:
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     prefix + "a",
							Version: 100,
						}, opts.StartCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}

	// cursor the same as prefix
	for _, prefix := range []metabase.ObjectKey{"a/", "a/a/", "/", "//"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := metabase.ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: metabase.ListObjectsCursor{
							Key:     prefix,
							Version: 100,
						},
						Prefix: prefix,
					}

					switch {
					case !allVersions:
						// latest version double check
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     prefix,
							Version: opts.FirstVersion(),
						}, opts.StartCursor(), opts)

					case allVersions:
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     prefix,
							Version: 100,
						}, opts.StartCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}

	// cursor before the prefix
	for _, cursor := range []metabase.ObjectKey{"", "a", "a/", "a/a", "b"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := metabase.ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: metabase.ListObjectsCursor{
							Key:     cursor,
							Version: 100,
						},
						Prefix: "b/",
					}

					assert.Equal(t, metabase.ListObjectsCursor{
						Key:     "b/",
						Version: opts.FirstVersion(),
					}, opts.StartCursor(), opts)
				}
			}
		}
	}

	// cursor after the prefix
	for _, cursor := range []metabase.ObjectKey{"c", "c/", "c/c"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := metabase.ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: metabase.ListObjectsCursor{
							Key:     cursor,
							Version: 100,
						},
						Prefix: "b/",
					}

					switch {
					case !allVersions:
						// latest version double check, optional
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     cursor,
							Version: opts.FirstVersion(),
						}, opts.StartCursor(), opts)

					case allVersions:
						assert.Equal(t, metabase.ListObjectsCursor{
							Key:     cursor,
							Version: 100,
						}, opts.StartCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}
}
