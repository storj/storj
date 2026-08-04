// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
)

var (
	TestProjectID = uuid.UUID([16]byte{0xd0, 0xfe, 0xe6, 0xc4, 0x12, 0x37, 0x42, 0x24, 0x96, 0x48, 0xcf, 0xab, 0xe3, 0x1f, 0x6e, 0x6f})
	TestBucket    = "bucket1"
	TestStreamID  = uuid.UUID([16]byte{0x93, 0x72, 0x6b, 0x8d, 0xd0, 0x4a, 0x45, 0xbb, 0x82, 0x4f, 0x67, 0x31, 0x86, 0xee, 0x6f, 0x96})
)

func TestCreateTestEvent(t *testing.T) {
	bucketName := "test-bucket"
	testEvent := CreateTestEvent(bucketName)

	assert.Equal(t, "Storj S3", testEvent.Service)
	assert.Equal(t, "s3:TestEvent", testEvent.Event)
	assert.Equal(t, bucketName, testEvent.Bucket)

	// Verify Time is in ISO8601 format and is recent
	require.NotEmpty(t, testEvent.Time)
	eventTime, err := time.Parse(ISO8601, testEvent.Time)
	assert.NoError(t, err, "Time should be in ISO8601 format")
	assert.WithinDuration(t, time.Now(), eventTime, 1*time.Second)

	// Verify Bytes returns valid JSON
	data, err := testEvent.Bytes()
	require.NoError(t, err)
	var roundtrip TestEvent
	require.NoError(t, json.Unmarshal(data, &roundtrip))
	assert.Equal(t, testEvent, roundtrip)
}

func TestValidateEventTypes(t *testing.T) {
	t.Run("Valid event types", func(t *testing.T) {
		// Individual specific event types
		err := ValidateEventTypes([]string{"s3:ObjectCreated:Put"})
		require.NoError(t, err)

		err = ValidateEventTypes([]string{"s3:ObjectCreated:Copy"})
		require.NoError(t, err)

		err = ValidateEventTypes([]string{"s3:ObjectRemoved:Delete"})
		require.NoError(t, err)

		err = ValidateEventTypes([]string{"s3:ObjectRemoved:DeleteMarkerCreated"})
		require.NoError(t, err)

		// Wildcard event types
		err = ValidateEventTypes([]string{"s3:ObjectCreated:*"})
		require.NoError(t, err)

		err = ValidateEventTypes([]string{"s3:ObjectRemoved:*"})
		require.NoError(t, err)

		// Multiple valid event types
		err = ValidateEventTypes([]string{
			"s3:ObjectCreated:Put",
			"s3:ObjectCreated:Copy",
			"s3:ObjectRemoved:Delete",
		})
		require.NoError(t, err)

		// Mix of specific and wildcard types
		err = ValidateEventTypes([]string{
			"s3:ObjectCreated:*",
			"s3:ObjectRemoved:Delete",
		})
		require.NoError(t, err)
	})

	t.Run("Invalid event types", func(t *testing.T) {
		// Invalid event type
		err := ValidateEventTypes([]string{"s3:ObjectCreated:Invalid"})
		require.True(t, ErrInvalidEventType.Has(err))

		// Missing s3: prefix
		err = ValidateEventTypes([]string{"ObjectCreated:Put"})
		require.True(t, ErrInvalidEventType.Has(err))

		// Wrong category
		err = ValidateEventTypes([]string{"s3:ObjectUpdated:Put"})
		require.True(t, ErrInvalidEventType.Has(err))

		// Mix of valid and invalid
		err = ValidateEventTypes([]string{
			"s3:ObjectCreated:Put",
			"s3:InvalidEvent:Type",
		})
		require.True(t, ErrInvalidEventType.Has(err))

		// Empty string
		err = ValidateEventTypes([]string{""})
		require.True(t, ErrInvalidEventType.Has(err))
	})

	t.Run("Empty event list", func(t *testing.T) {
		err := ValidateEventTypes([]string{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least one event type is required")

		err = ValidateEventTypes(nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least one event type is required")
	})
}

func TestMatchEventType(t *testing.T) {
	t.Run("Exact match with s3: prefix", func(t *testing.T) {
		configuredEvents := []string{"s3:ObjectCreated:Put", "s3:ObjectRemoved:Delete"}

		assert.True(t, MatchEventType("s3:ObjectCreated:Put", configuredEvents))
		assert.True(t, MatchEventType("s3:ObjectRemoved:Delete", configuredEvents))
	})

	t.Run("Exact match - event without s3: prefix (normalized)", func(t *testing.T) {
		configuredEvents := []string{"s3:ObjectCreated:Put", "s3:ObjectRemoved:Delete"}

		// Event type without s3: prefix should be normalized by adding it
		assert.True(t, MatchEventType("ObjectCreated:Put", configuredEvents))
		assert.True(t, MatchEventType("ObjectRemoved:Delete", configuredEvents))
	})

	t.Run("Wildcard match - ObjectCreated:*", func(t *testing.T) {
		configuredEvents := []string{"s3:ObjectCreated:*"}

		assert.True(t, MatchEventType("s3:ObjectCreated:Put", configuredEvents))
		assert.True(t, MatchEventType("s3:ObjectCreated:Copy", configuredEvents))
		assert.True(t, MatchEventType("ObjectCreated:Put", configuredEvents))
		assert.True(t, MatchEventType("ObjectCreated:Copy", configuredEvents))

		// Should not match different category
		assert.False(t, MatchEventType("s3:ObjectRemoved:Delete", configuredEvents))
	})

	t.Run("Wildcard match - ObjectRemoved:*", func(t *testing.T) {
		configuredEvents := []string{"s3:ObjectRemoved:*"}

		assert.True(t, MatchEventType("s3:ObjectRemoved:Delete", configuredEvents))
		assert.True(t, MatchEventType("s3:ObjectRemoved:DeleteMarkerCreated", configuredEvents))
		assert.True(t, MatchEventType("ObjectRemoved:Delete", configuredEvents))
		assert.True(t, MatchEventType("ObjectRemoved:DeleteMarkerCreated", configuredEvents))

		// Should not match different category
		assert.False(t, MatchEventType("s3:ObjectCreated:Put", configuredEvents))
	})

	t.Run("Multiple configured events", func(t *testing.T) {
		configuredEvents := []string{
			"s3:ObjectCreated:Put",
			"s3:ObjectRemoved:*",
		}

		assert.True(t, MatchEventType("s3:ObjectCreated:Put", configuredEvents))
		assert.True(t, MatchEventType("s3:ObjectRemoved:Delete", configuredEvents))
		assert.True(t, MatchEventType("s3:ObjectRemoved:DeleteMarkerCreated", configuredEvents))

		assert.False(t, MatchEventType("s3:ObjectCreated:Copy", configuredEvents))
	})

	t.Run("No match", func(t *testing.T) {
		configuredEvents := []string{"s3:ObjectCreated:Put"}

		assert.False(t, MatchEventType("s3:ObjectCreated:Copy", configuredEvents))
		assert.False(t, MatchEventType("s3:ObjectRemoved:Delete", configuredEvents))
	})

	t.Run("Empty configured events", func(t *testing.T) {
		assert.False(t, MatchEventType("s3:ObjectCreated:Put", []string{}))
		assert.False(t, MatchEventType("s3:ObjectCreated:Put", nil))
	})

	t.Run("Empty event type", func(t *testing.T) {
		configuredEvents := []string{"s3:ObjectCreated:Put"}
		// Empty event type gets normalized to "s3:" which won't match anything
		assert.False(t, MatchEventType("", configuredEvents))
	})
}

func TestMatchFilters(t *testing.T) {
	t.Run("No filters - all objects match", func(t *testing.T) {
		assert.True(t, MatchFilters([]byte("any/object/key"), nil, nil))
		assert.True(t, MatchFilters([]byte("any/object/key"), []byte{}, []byte{}))
		assert.True(t, MatchFilters([]byte(""), []byte{}, []byte{}))
	})

	t.Run("Prefix filter only", func(t *testing.T) {
		filterPrefix := []byte("images/")

		assert.True(t, MatchFilters([]byte("images/photo.jpg"), filterPrefix, nil))
		assert.True(t, MatchFilters([]byte("images/subfolder/photo.jpg"), filterPrefix, nil))
		assert.True(t, MatchFilters([]byte("images/"), filterPrefix, nil))

		assert.False(t, MatchFilters([]byte("videos/movie.mp4"), filterPrefix, nil))
		assert.False(t, MatchFilters([]byte("image.jpg"), filterPrefix, nil))
		assert.False(t, MatchFilters([]byte(""), filterPrefix, nil))
	})

	t.Run("Suffix filter only", func(t *testing.T) {
		filterSuffix := []byte(".jpg")

		assert.True(t, MatchFilters([]byte("photo.jpg"), nil, filterSuffix))
		assert.True(t, MatchFilters([]byte("folder/photo.jpg"), nil, filterSuffix))
		assert.True(t, MatchFilters([]byte("images/subfolder/pic.jpg"), nil, filterSuffix))

		assert.False(t, MatchFilters([]byte("photo.png"), nil, filterSuffix))
		assert.False(t, MatchFilters([]byte("document.txt"), nil, filterSuffix))
		assert.False(t, MatchFilters([]byte(""), nil, filterSuffix))
	})

	t.Run("Both prefix and suffix filters", func(t *testing.T) {
		filterPrefix := []byte("images/")
		filterSuffix := []byte(".jpg")

		assert.True(t, MatchFilters([]byte("images/photo.jpg"), filterPrefix, filterSuffix))
		assert.True(t, MatchFilters([]byte("images/subfolder/pic.jpg"), filterPrefix, filterSuffix))

		assert.False(t, MatchFilters([]byte("images/photo.png"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte("videos/photo.jpg"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte("photo.jpg"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte(""), filterPrefix, filterSuffix))
	})

	t.Run("Empty prefix filter (treated as no filter)", func(t *testing.T) {
		filterSuffix := []byte(".jpg")

		assert.True(t, MatchFilters([]byte("photo.jpg"), []byte{}, filterSuffix))
		assert.True(t, MatchFilters([]byte("folder/photo.jpg"), []byte{}, filterSuffix))
	})

	t.Run("Empty suffix filter (treated as no filter)", func(t *testing.T) {
		filterPrefix := []byte("images/")

		assert.True(t, MatchFilters([]byte("images/photo.jpg"), filterPrefix, []byte{}))
		assert.True(t, MatchFilters([]byte("images/photo.png"), filterPrefix, []byte{}))
	})

	t.Run("Single character filters", func(t *testing.T) {
		assert.True(t, MatchFilters([]byte("a/test"), []byte("a"), nil))
		assert.True(t, MatchFilters([]byte("test.a"), nil, []byte("a")))
		assert.False(t, MatchFilters([]byte("b/test"), []byte("a"), nil))
		assert.False(t, MatchFilters([]byte("test.b"), nil, []byte("a")))
	})

	t.Run("Special characters in filters", func(t *testing.T) {
		filterPrefix := []byte("logs/2024-01-")
		filterSuffix := []byte(".log.gz")

		assert.True(t, MatchFilters([]byte("logs/2024-01-15.log.gz"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte("logs/2024-02-15.log.gz"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte("logs/2024-01-15.log"), filterPrefix, filterSuffix))
	})

	t.Run("Unicode characters", func(t *testing.T) {
		filterPrefix := []byte("données/")
		filterSuffix := []byte(".txt")

		assert.True(t, MatchFilters([]byte("données/fichier.txt"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte("data/fichier.txt"), filterPrefix, filterSuffix))
	})

	t.Run("Empty object key", func(t *testing.T) {
		assert.True(t, MatchFilters([]byte(""), nil, nil))
		assert.True(t, MatchFilters([]byte(""), []byte{}, []byte{}))
		assert.False(t, MatchFilters([]byte(""), []byte("prefix"), nil))
		assert.False(t, MatchFilters([]byte(""), nil, []byte("suffix")))
	})

	t.Run("Case sensitivity", func(t *testing.T) {
		filterPrefix := []byte("Images/")
		filterSuffix := []byte(".JPG")

		// Exact case match - should pass
		assert.True(t, MatchFilters([]byte("Images/photo.JPG"), filterPrefix, filterSuffix))

		// Different case - should fail (filters are case-sensitive)
		assert.False(t, MatchFilters([]byte("images/photo.JPG"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte("Images/photo.jpg"), filterPrefix, filterSuffix))
		assert.False(t, MatchFilters([]byte("IMAGES/photo.JPG"), filterPrefix, filterSuffix))
	})
}

func TestEncodeForS3Event(t *testing.T) {
	for _, tt := range []struct {
		input    string
		expected string
	}{
		// Plain keys need no encoding
		{"simple.txt", "simple.txt"},
		{"folder/file.txt", "folder/file.txt"},
		// Spaces become "+"
		{"my file.txt", "my+file.txt"},
		{"folder/my file.txt", "folder/my+file.txt"},
		// Literal "+" becomes "%2B"
		{"file+name.txt", "file%2Bname.txt"},
		// Both space and "+" in the same key
		{"my file+test (1).txt", "my+file%2Btest+%281%29.txt"},
		// Path delimiters "/" are preserved
		{"file name with spaces/sub dir/file.txt", "file+name+with+spaces/sub+dir/file.txt"},
		// Special characters are percent-encoded
		{"file&name.txt", "file%26name.txt"},
		// Empty key
		{"", ""},
	} {
		assert.Equal(t, tt.expected, string(EncodeForS3Event([]byte(tt.input))), "input: %q", tt.input)
	}
}
