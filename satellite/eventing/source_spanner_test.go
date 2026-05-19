// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractString(t *testing.T) {
	values := map[string]any{
		"string_key": "test_value",
		"int_key":    123,
		"nil_key":    nil,
	}

	t.Run("Valid string extraction", func(t *testing.T) {
		result, ok := extractString("string_key", values)
		require.True(t, ok)
		require.Equal(t, "test_value", result)
	})

	t.Run("Non-string value", func(t *testing.T) {
		result, ok := extractString("int_key", values)
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Missing key", func(t *testing.T) {
		result, ok := extractString("missing_key", values)
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Nil values map", func(t *testing.T) {
		result, ok := extractString("any_key", nil)
		require.False(t, ok)
		require.Equal(t, "", result)
	})
}

func TestExtractInt64(t *testing.T) {
	values := map[string]any{
		"int64_key":    int64(123),
		"float64_key":  float64(456.0),
		"string_key":   "not_a_number",
		"json_num_key": json.Number("789"),
	}

	t.Run("Valid int64 extraction", func(t *testing.T) {
		result, ok := extractInt64("int64_key", values)
		require.True(t, ok)
		require.Equal(t, int64(123), result)
	})

	t.Run("Valid float64 extraction", func(t *testing.T) {
		result, ok := extractInt64("float64_key", values)
		require.True(t, ok)
		require.Equal(t, int64(456), result)
	})

	t.Run("Valid json.Number extraction", func(t *testing.T) {
		result, ok := extractInt64("json_num_key", values)
		require.True(t, ok)
		require.Equal(t, int64(789), result)
	})

	t.Run("Non-numeric value", func(t *testing.T) {
		result, ok := extractInt64("string_key", values)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})

	t.Run("Missing key", func(t *testing.T) {
		result, ok := extractInt64("missing_key", values)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})

	t.Run("Nil values map", func(t *testing.T) {
		result, ok := extractInt64("any_key", nil)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})
}

func TestExtractFirst(t *testing.T) {
	values1 := map[string]any{
		"string_key": "first_value",
		"int64_key":  int64(123),
		"nil_key":    nil,
	}

	values2 := map[string]any{
		"string_key": "second_value",
		"int64_key":  int64(321),
		"nil_key":    nil,
	}

	t.Run("Valid string extraction", func(t *testing.T) {
		result, ok := extractFirstString("string_key", values1, values2)
		require.True(t, ok)
		require.Equal(t, "first_value", result)
	})

	t.Run("Valid string extraction, first nil map", func(t *testing.T) {
		result, ok := extractFirstString("string_key", nil, values2)
		require.True(t, ok)
		require.Equal(t, "second_value", result)
	})

	t.Run("Valid int64 extraction", func(t *testing.T) {
		result, ok := extractFirstInt64("int64_key", values1, values2)
		require.True(t, ok)
		require.Equal(t, int64(123), result)
	})

	t.Run("Valid int64 extraction, first nil map", func(t *testing.T) {
		result, ok := extractFirstInt64("int64_key", nil, values2)
		require.True(t, ok)
		require.Equal(t, int64(321), result)
	})

	t.Run("Nil values map, string", func(t *testing.T) {
		result, ok := extractFirstString("any_key", nil, nil)
		require.False(t, ok)
		require.Equal(t, "", result)
	})

	t.Run("Nil values map, int64", func(t *testing.T) {
		result, ok := extractFirstInt64("any_key", nil, nil)
		require.False(t, ok)
		require.Equal(t, int64(0), result)
	})
}
