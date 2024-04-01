// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
				opts := ListObjects{
					AllVersions: allVersions,
					Recursive:   recursive,
					Pending:     pending,
					Cursor:      ListObjectsCursor{},
				}

				switch {
				case !allVersions:
					// latest version double check, optional
					assert.Equal(t, ListObjectsCursor{
						Key:     "",
						Version: opts.firstVersion(),
					}, opts.startCursor(), opts)
				default:
					assert.Equal(t, ListObjectsCursor{
						Key:     "",
						Version: 0,
					}, opts.startCursor(), opts)
				}
			}
		}
	}

	// plain simple cursor
	for _, allVersions := range []bool{false, true} {
		for _, recursive := range []bool{false, true} {
			for _, pending := range []bool{false, true} {
				opts := ListObjects{
					AllVersions: allVersions,
					Recursive:   recursive,
					Pending:     pending,
					Cursor: ListObjectsCursor{
						Key:     "a",
						Version: 100,
					},
				}

				switch {
				case !allVersions:
					// latest version double check
					assert.Equal(t, ListObjectsCursor{
						Key:     "a",
						Version: opts.firstVersion(),
					}, opts.startCursor(), opts)

				case allVersions:
					assert.Equal(t, ListObjectsCursor{
						Key:     "a",
						Version: 100,
					}, opts.startCursor(), opts)

				default:
					panic("unhandled scenario")
				}
			}
		}
	}

	// cursor with nesting
	for _, prefix := range []ObjectKey{"", "x/", "x/x/", "/", "//"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: ListObjectsCursor{
							Key:     prefix + "a/a",
							Version: 100,
						},
						Prefix: prefix,
					}

					switch {
					case recursive && allVersions:
						assert.Equal(t, ListObjectsCursor{
							Key:     prefix + "a/a",
							Version: 100,
						}, opts.startCursor(), opts)

					case recursive && !allVersions:
						// latest version double check
						assert.Equal(t, ListObjectsCursor{
							Key:     prefix + "a/a",
							Version: opts.firstVersion(),
						}, opts.startCursor(), opts)

					case !recursive:
						// skip outside for non-recursive
						assert.Equal(t, ListObjectsCursor{
							Key:     prefix + "a" + DelimiterNext,
							Version: opts.firstVersion(),
						}, opts.startCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}

	// cursor inside a prefix
	for _, prefix := range []ObjectKey{"a/", "a/a/", "/", "//"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: ListObjectsCursor{
							Key:     prefix + "a",
							Version: 100,
						},
						Prefix: prefix,
					}

					switch {
					case !allVersions:
						// latest version double check
						assert.Equal(t, ListObjectsCursor{
							Key:     prefix + "a",
							Version: opts.firstVersion(),
						}, opts.startCursor(), opts)

					case allVersions:
						assert.Equal(t, ListObjectsCursor{
							Key:     prefix + "a",
							Version: 100,
						}, opts.startCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}

	// cursor the same as prefix
	for _, prefix := range []ObjectKey{"a/", "a/a/", "/", "//"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: ListObjectsCursor{
							Key:     prefix,
							Version: 100,
						},
						Prefix: prefix,
					}

					switch {
					case !allVersions:
						// latest version double check
						assert.Equal(t, ListObjectsCursor{
							Key:     prefix,
							Version: opts.firstVersion(),
						}, opts.startCursor(), opts)

					case allVersions:
						assert.Equal(t, ListObjectsCursor{
							Key:     prefix,
							Version: 100,
						}, opts.startCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}

	// cursor before the prefix
	for _, cursor := range []ObjectKey{"", "a", "a/", "a/a", "b"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: ListObjectsCursor{
							Key:     cursor,
							Version: 100,
						},
						Prefix: "b/",
					}

					assert.Equal(t, ListObjectsCursor{
						Key:     "b/",
						Version: opts.firstVersion(),
					}, opts.startCursor(), opts)
				}
			}
		}
	}

	// cursor after the prefix
	for _, cursor := range []ObjectKey{"c", "c/", "c/c"} {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor: ListObjectsCursor{
							Key:     cursor,
							Version: 100,
						},
						Prefix: "b/",
					}

					switch {
					case !allVersions:
						// latest version double check, optional
						assert.Equal(t, ListObjectsCursor{
							Key:     cursor,
							Version: opts.firstVersion(),
						}, opts.startCursor(), opts)

					case allVersions:
						assert.Equal(t, ListObjectsCursor{
							Key:     cursor,
							Version: 100,
						}, opts.startCursor(), opts)

					default:
						panic("unhandled scenario")
					}
				}
			}
		}
	}
}
