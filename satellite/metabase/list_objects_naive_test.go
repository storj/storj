// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/metabase"
)

// NaiveObjectsDB implements a slow reference ListObjects implementation.
type NaiveObjectsDB struct {
	VersionAsc  []metabase.ObjectEntry
	VersionDesc []metabase.ObjectEntry
}

// NewNaiveObjectsDB returns a new NaiveObjectsDB.
func NewNaiveObjectsDB(rawentries []metabase.ObjectEntry) *NaiveObjectsDB {
	versiondesc := slices.Clone(rawentries)
	sort.Slice(versiondesc, func(i, k int) bool {
		return versiondesc[i].Less(versiondesc[k])
	})

	entries := make([]metabase.ObjectEntry, 0, len(rawentries))

	lastEntry := metabase.ObjectKey("")
	for _, entry := range versiondesc {
		if entry.Status != metabase.Pending {
			entry.IsLatest = entry.ObjectKey != lastEntry
			lastEntry = entry.ObjectKey
		}
		entries = append(entries, entry)
	}

	db := &NaiveObjectsDB{
		VersionAsc:  slices.Clone(entries),
		VersionDesc: entries,
	}

	sort.Slice(db.VersionAsc, func(i, k int) bool {
		return db.VersionAsc[i].LessVersionAsc(db.VersionAsc[k])
	})
	return db
}

// ListObjects lists objects.
func (db *NaiveObjectsDB) ListObjects(ctx context.Context, opts metabase.ListObjects) (result metabase.ListObjectsResult, err error) {
	metabase.ListLimit.Ensure(&opts.Limit)
	if opts.Delimiter == "" {
		opts.Delimiter = metabase.Delimiter
	}

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
		if entryExcludedByCursor(&opts, entryKeyWithPrefix, entryVersion, isPrefix) {
			continue
		}

		if opts.Pending != (entry.Status == metabase.Pending) {
			continue
		}

		// AllVersions=false should only care about the latest versions.
		if !opts.AllVersions && !entry.IsLatest {
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
func entryExcludedByCursor(opts *metabase.ListObjects, entryKeyWithPrefix metabase.ObjectKey, entryVersion metabase.Version, isPrefix bool) bool {
	if opts.Cursor.Key == entryKeyWithPrefix && !isPrefix {
		if opts.VersionAscending() {
			return entryVersion <= opts.Cursor.Version
		} else {
			return entryVersion >= opts.Cursor.Version
		}
	}
	return entryKeyWithPrefix <= opts.Cursor.Key
}

func calculateEntryKey(opts *metabase.ListObjects, entry *metabase.ObjectEntry) (entryKeyWithPrefix, entryKey metabase.ObjectKey, entryVersion metabase.Version, isPrefix bool) {
	entryKey = entry.ObjectKey[len(opts.Prefix):]

	if !opts.Recursive {
		if i := strings.Index(string(entryKey), string(opts.Delimiter)); i >= 0 {
			return entry.ObjectKey[:len(opts.Prefix)+i+len(opts.Delimiter)], entryKey[:i+len(opts.Delimiter)], 0, true
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

	if !opts.IncludeCustomMetadata && !opts.IncludeETag && !opts.IncludeETagOrCustomMetadata {
		entry.EncryptedMetadataNonce = nil
		entry.EncryptedMetadataEncryptedKey = nil
	}

	if opts.IncludeETagOrCustomMetadata {
		if len(entry.EncryptedETag) > 0 {
			if !opts.IncludeCustomMetadata {
				entry.EncryptedMetadata = nil
			}
		}
	} else {
		if !opts.IncludeCustomMetadata {
			entry.EncryptedMetadata = nil
		}
		if !opts.IncludeETag {
			entry.EncryptedETag = nil
		}
	}

}

func TestNaiveObjectsDB_Basic(t *testing.T) {
	check := func(entries []metabase.ObjectEntry, opts metabase.ListObjects, expected []metabase.ObjectEntry) {
		t.Helper()
		naive := NewNaiveObjectsDB(entries)
		result, err := naive.ListObjects(t.Context(), opts)
		require.NoError(t, err)
		require.Equal(t, expected, result.Objects)
	}

	check(
		[]metabase.ObjectEntry{
			{ObjectKey: "a/a", Version: 1, Status: metabase.CommittedVersioned},
			{ObjectKey: "a/b", Version: 1, Status: metabase.CommittedVersioned},
			{ObjectKey: "b", Version: 1, Status: metabase.CommittedVersioned},
		},
		metabase.ListObjects{
			AllVersions: false,
			Recursive:   false,
			Pending:     false,
			Limit:       2,
			Prefix:      "",
			Cursor:      metabase.ListObjectsCursor{Key: "a/", Version: 0},
		},
		[]metabase.ObjectEntry{
			{ObjectKey: "b", Version: 1, Status: metabase.CommittedVersioned, IsLatest: true},
		},
	)

	check(
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00", Version: 3, Status: metabase.CommittedVersioned},
			{ObjectKey: "\x00\x00", Version: 1, Status: metabase.CommittedVersioned},
		},
		metabase.ListObjects{
			AllVersions: true,
			Recursive:   true,
			Pending:     false,
			Limit:       1,
			Prefix:      "",
			Cursor:      metabase.ListObjectsCursor{Key: "\x00", Version: 0},
		},
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00\x00", Version: 1, Status: metabase.CommittedVersioned, IsLatest: true},
		},
	)

	check(
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00/", Version: 10, Status: metabase.CommittedVersioned},
			{ObjectKey: "\x00\xff", Version: 5, Status: metabase.CommittedVersioned},
		},
		metabase.ListObjects{
			AllVersions: true,
			Recursive:   false,
			Pending:     false,
			Limit:       1,
			Prefix:      "",
			Cursor:      metabase.ListObjectsCursor{Key: "\x00/", Version: 0},
		},
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00\xff", Version: 5, Status: metabase.CommittedVersioned, IsLatest: true},
		},
	)

	check(
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00/", Version: 10, Status: metabase.CommittedVersioned},
			{ObjectKey: "\x00\xff", Version: 5, Status: metabase.CommittedVersioned},
		},
		metabase.ListObjects{
			AllVersions: true,
			Recursive:   false,
			Pending:     false,
			Limit:       2,
			Prefix:      "",
			Cursor:      metabase.ListObjectsCursor{Key: "\x00/", Version: metabase.MaxVersion},
		},
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00\xff", Version: 5, Status: metabase.CommittedVersioned, IsLatest: true},
		},
	)

	check(
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00/", Version: 10, Status: metabase.CommittedVersioned},
			{ObjectKey: "\x00\xff", Version: 5, Status: metabase.CommittedVersioned},
		},
		metabase.ListObjects{
			AllVersions: true,
			Recursive:   false,
			Pending:     false,
			Limit:       2,
			Prefix:      "",
			Cursor:      metabase.ListObjectsCursor{Key: "\x00", Version: 0},
		},
		[]metabase.ObjectEntry{
			{ObjectKey: "\x00/", Version: 0, Status: metabase.Prefix, IsPrefix: true},
			{ObjectKey: "\x00\xff", Version: 5, Status: metabase.CommittedVersioned, IsLatest: true},
		},
	)
}
