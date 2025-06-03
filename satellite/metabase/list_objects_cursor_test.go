// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	t.Run("Unskippable prefix in starting cursor", func(t *testing.T) {
		opts := metabase.ListObjects{
			Prefix:    "a",
			Delimiter: "\xff",
			Cursor: metabase.ListObjectsCursor{
				Key:     "a\xffb",
				Version: 100,
			},
		}

		startCursor, ok := opts.StartCursor()
		assert.False(t, ok)
		assert.Empty(t, startCursor)
	})

	t.Run("Empty starting cursor", func(t *testing.T) {
		for _, allVersions := range []bool{false, true} {
			for _, recursive := range []bool{false, true} {
				for _, pending := range []bool{false, true} {
					opts := metabase.ListObjects{
						AllVersions: allVersions,
						Recursive:   recursive,
						Pending:     pending,
						Cursor:      metabase.ListObjectsCursor{},
					}

					startCursor, ok := opts.StartCursor()
					assert.True(t, ok)

					// latest version double check
					assert.Equal(t, metabase.ListObjectsCursor{
						Key:     "",
						Version: opts.FirstVersion(),
					}, startCursor, opts)
				}
			}
		}
	})

	t.Run("Simple cursor", func(t *testing.T) {
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

					startCursor, ok := opts.StartCursor()
					assert.True(t, ok)

					// latest version double check
					assert.Equal(t, metabase.ListObjectsCursor{
						Key:     "a",
						Version: opts.FirstVersion(),
					}, startCursor, opts)
				}
			}
		}
	})

	t.Run("Cursor with nesting", func(t *testing.T) {
		for _, delimiter := range []metabase.ObjectKey{"/", "DELIM"} {
			for _, prefix := range []metabase.ObjectKey{
				"",
				"x" + delimiter,
				"x" + delimiter + "x" + delimiter,
				delimiter,
				delimiter + delimiter,
			} {
				nextDelimiter, ok := metabase.SkipPrefix(delimiter)
				require.True(t, ok)

				for _, allVersions := range []bool{false, true} {
					for _, recursive := range []bool{false, true} {
						for _, pending := range []bool{false, true} {
							opts := metabase.ListObjects{
								AllVersions: allVersions,
								Recursive:   recursive,
								Pending:     pending,
								Cursor: metabase.ListObjectsCursor{
									Key:     prefix + "a" + delimiter + "a",
									Version: 100,
								},
								Prefix:    prefix,
								Delimiter: delimiter,
							}

							startCursor, ok := opts.StartCursor()
							assert.True(t, ok)

							switch {
							case recursive:
								// latest version double check
								assert.Equal(t, metabase.ListObjectsCursor{
									Key:     prefix + "a" + delimiter + "a",
									Version: opts.FirstVersion(),
								}, startCursor, opts)

							case !recursive:
								// skip outside for non-recursive
								assert.Equal(t, metabase.ListObjectsCursor{
									Key:     prefix + "a" + nextDelimiter,
									Version: opts.FirstVersion(),
								}, startCursor, opts)

							default:
								panic("unhandled scenario")
							}
						}
					}
				}
			}
		}
	})

	t.Run("Cursor inside a prefix", func(t *testing.T) {
		for _, delimiter := range []metabase.ObjectKey{"/", "DELIM"} {
			for _, prefix := range []metabase.ObjectKey{
				"a" + delimiter,
				"a" + delimiter + "a" + delimiter,
				delimiter,
				delimiter + delimiter,
			} {
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

							startCursor, ok := opts.StartCursor()
							assert.True(t, ok)

							// latest version double check
							assert.Equal(t, metabase.ListObjectsCursor{
								Key:     prefix + "a",
								Version: opts.FirstVersion(),
							}, startCursor, opts)
						}
					}
				}
			}
		}
	})

	t.Run("Cursor the same as prefix", func(t *testing.T) {
		for _, delimiter := range []metabase.ObjectKey{"/", "DELIM"} {
			for _, prefix := range []metabase.ObjectKey{
				"a" + delimiter,
				"a" + delimiter + "a" + delimiter,
				delimiter,
				delimiter + delimiter,
			} {
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

							startCursor, ok := opts.StartCursor()
							assert.True(t, ok)

							// latest version double check
							assert.Equal(t, metabase.ListObjectsCursor{
								Key:     prefix,
								Version: opts.FirstVersion(),
							}, startCursor, opts)
						}
					}
				}
			}
		}
	})

	t.Run("Cursor before the prefix", func(t *testing.T) {
		for _, delimiter := range []metabase.ObjectKey{"/", "DELIM"} {
			for _, cursor := range []metabase.ObjectKey{
				"",
				"a",
				"a" + delimiter,
				"a" + delimiter + "a",
				"b",
			} {
				nextDelimiter, ok := metabase.SkipPrefix(delimiter)
				require.True(t, ok)

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
								Prefix: "b" + nextDelimiter,
							}

							startCursor, ok := opts.StartCursor()
							assert.True(t, ok)

							assert.Equal(t, metabase.ListObjectsCursor{
								Key:     "b" + nextDelimiter,
								Version: opts.FirstVersion(),
							}, startCursor, opts)
						}
					}
				}
			}
		}
	})

	t.Run("Cursor after the prefix", func(t *testing.T) {
		for _, delimiter := range []metabase.ObjectKey{"/", "DELIM"} {
			for _, cursor := range []metabase.ObjectKey{"c", "c" + delimiter, "c" + delimiter + "c"} {
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
								Prefix: "b" + delimiter,
							}

							startCursor, ok := opts.StartCursor()
							assert.True(t, ok)

							// latest version double check
							assert.Equal(t, metabase.ListObjectsCursor{
								Key:     cursor,
								Version: opts.FirstVersion(),
							}, startCursor, opts)
						}
					}
				}
			}
		}
	})
}
