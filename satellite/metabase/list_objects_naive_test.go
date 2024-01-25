// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"storj.io/storj/satellite/metabase"
)

// NaiveObjectsDB implements a slow reference ListObjects implementation.
type NaiveObjectsDB struct {
	VersionAsc  []metabase.ObjectEntry
	VersionDesc []metabase.ObjectEntry
}

// NewNaiveObjectsDB returns a new NaiveObjectsDB.
func NewNaiveObjectsDB(entries []metabase.ObjectEntry) *NaiveObjectsDB {
	db := &NaiveObjectsDB{
		VersionAsc:  slices.Clone(entries),
		VersionDesc: slices.Clone(entries),
	}
	sort.Slice(db.VersionAsc, func(i, k int) bool {
		return db.VersionAsc[i].LessVersionAsc(db.VersionAsc[k])
	})
	sort.Slice(db.VersionDesc, func(i, k int) bool {
		return db.VersionDesc[i].Less(db.VersionDesc[k])
	})
	return db
}

// ListObjects lists objects.
func (db *NaiveObjectsDB) ListObjects(ctx context.Context, opts metabase.ListObjects) (result metabase.ListObjectsResult, err error) {
	metabase.ListLimit.Ensure(&opts.Limit)

	entries := db.VersionDesc
	if opts.Pending {
		entries = db.VersionAsc
	}

	var last *metabase.ObjectEntry
	for i := range entries {
		entry := &entries[i]

		if !strings.HasPrefix(string(entry.ObjectKey), string(opts.Prefix)) {
			if opts.Prefix < entry.ObjectKey {
				// we went past the prefix, no more potential matches
				break
			}
			continue
		}

		// remove opts.Prefix and collapse child key
		entryKeyWithPrefix, entryKey, entryVersion, isPrefix := calculateEntryKey(&opts, entry)

		// The entry is before our cursor position.
		if !lessIterateCursor(opts.Cursor, entryKeyWithPrefix, entryVersion) {
			continue
		}

		if opts.Pending != (entry.Status == metabase.Pending) {
			continue
		}

		if last != nil {
			if isPrefix {
				// prefix already included in output
				if entryKey == last.ObjectKey && last.IsPrefix {
					continue
				}
			} else {
				// version already included
				if !opts.AllVersions && !last.IsPrefix && entryKey == last.ObjectKey {
					continue
				}
			}
		}

		var scoped metabase.ObjectEntry
		if isPrefix {
			scoped.ObjectKey = entryKey
			scoped.IsPrefix = true
			scoped.Status = metabase.Prefix
		} else {
			scoped = *entry
			scoped.ObjectKey = entryKey
			clearEntryMetadata(&opts, &scoped)
		}

		if !opts.AllVersions && scoped.Status.IsDeleteMarker() {
			// We don't want to include delete markers in output,
			// however, we do want to skip entries, if they have the same key.
			last = &scoped
			continue
		}
		result.Objects = append(result.Objects, scoped)
		last = &result.Objects[len(result.Objects)-1]

		if len(result.Objects) >= opts.Limit+1 {
			result.More = true
			result.Objects = result.Objects[:opts.Limit]
			return result, nil
		}
	}

	return result, nil
}

// Less returns whether key and version are after the cursor.
func lessIterateCursor(cursor metabase.ListObjectsCursor, key metabase.ObjectKey, version metabase.Version) bool {
	if cursor.Key == key {
		return cursor.Version < version
	}
	return cursor.Key < key
}

func calculateEntryKey(opts *metabase.ListObjects, entry *metabase.ObjectEntry) (entryKeyWithPrefix, entryKey metabase.ObjectKey, entryVersion metabase.Version, isPrefix bool) {
	entryKey = entry.ObjectKey[len(opts.Prefix):]

	if !opts.Recursive {
		if i := strings.IndexByte(string(entryKey), '/'); i >= 0 {
			return entry.ObjectKey[:len(opts.Prefix)+i+1], entryKey[:i+1], 0, true
		}
	}

	return entry.ObjectKey, entryKey, entry.Version, false
}

func clearEntryMetadata(opts *metabase.ListObjects, entry *metabase.ObjectEntry) {
	if entry.IsPrefix {
		return
	}

	if !opts.IncludeSystemMetadata {
		entry.CreatedAt = time.Time{}
		entry.ExpiresAt = nil
		entry.SegmentCount = 0
		entry.TotalPlainSize = 0
		entry.TotalEncryptedSize = 0
		entry.FixedSegmentSize = 0
	}

	if !opts.IncludeCustomMetadata {
		entry.EncryptedMetadataNonce = nil
		entry.EncryptedMetadata = nil
		entry.EncryptedMetadataEncryptedKey = nil
	}
}

func TestNaiveObjectsDB_Basic(t *testing.T) {
	check := func(entries []metabase.ObjectEntry, opts metabase.ListObjects, expected []metabase.ObjectEntry) {
		t.Run("", func(t *testing.T) {
			naive := NewNaiveObjectsDB(entries)
			result, err := naive.ListObjects(context.Background(), opts)
			require.NoError(t, err)
			require.Equal(t, expected, result.Objects)
		})
	}

	check(
		[]metabase.ObjectEntry{
			{ObjectKey: "a/a", Version: 1, Status: metabase.CommittedVersioned},
			{ObjectKey: "a/b", Version: 1, Status: metabase.CommittedVersioned},
			{ObjectKey: "b", Version: 1, Status: metabase.CommittedVersioned},
		},
		metabase.ListObjects{
			Recursive: false,
			Limit:     2,
			Prefix:    "",
			Cursor:    metabase.ListObjectsCursor{Key: "a/", Version: 0},
		},
		[]metabase.ObjectEntry{
			{ObjectKey: "b", Version: 1, Status: metabase.CommittedVersioned},
		},
	)
}
