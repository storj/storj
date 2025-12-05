// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zeebo/assert"
)

type parseTest[T any] struct {
	name string
	data T
	ok   bool
}

func runParseHelperTest[T any](t *testing.T, fn func(string) (T, bool), tests []parseTest[T]) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok := fn(tt.name)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, data, tt.data)
		})
	}
}

func TestParseHelpers(t *testing.T) {
	t.Run("parseLog", func(t *testing.T) {
		runParseHelperTest(t, parseLog, []parseTest[logInfo]{
			// Valid test cases
			{name: "log-0000000000000001", data: logInfo{id: 1, ttl: 0}, ok: true},
			{name: "log-0000000000000002-0000ffff", data: logInfo{id: 2, ttl: 0xffff}, ok: true},
			{name: "log-ffffffffffffffff", data: logInfo{id: 0xffffffffffffffff, ttl: 0}, ok: true},
			{name: "log-0000000000000003-ffffffff", data: logInfo{id: 3, ttl: 0xffffffff}, ok: true},

			// Test cases with capital letters in hex digits (should work - hex is case-insensitive)
			{name: "log-000000000000000A", data: logInfo{id: 0xa, ttl: 0}, ok: true},
			{name: "log-000000000000000F", data: logInfo{id: 0xf, ttl: 0}, ok: true},
			{name: "log-FFFFFFFFFFFFFFFF", data: logInfo{id: 0xffffffffffffffff, ttl: 0}, ok: true},
			{name: "log-0000000000000010-0000ABCD", data: logInfo{id: 0x10, ttl: 0xabcd}, ok: true},
			{name: "log-00000000ABCDEF12", data: logInfo{id: 0xabcdef12, ttl: 0}, ok: true},
			{name: "log-0000000000000020-FFFFFFFF", data: logInfo{id: 0x20, ttl: 0xffffffff}, ok: true},

			// Test cases with invalid hex digits
			{name: "log-000000000000000g"},
			{name: "log-0000000000000001-0000000g"},

			// Test cases with capital letters in prefix (should NOT match - prefix is case-sensitive)
			{name: "Log-0000000000000001"},
			{name: "LOG-0000000000000001"},
			{name: "Log-FFFFFFFFFFFFFFFF"},
			{name: "loG-0000000000000001"},

			// Test cases with invalid lengths or prefixes
			{name: "logs-0000000000000001"},
			{name: "log-000000000000001"},
			{name: "log-00000000000000001"},
			{name: "log-0000000000000001-000000"},
			{name: "log-00000000000000010000ffff"},
			{name: "hashtbl-0000000000000001"},
			{name: "random.txt"},
		})
	})

	t.Run("parseHashtbl", func(t *testing.T) {
		runParseHelperTest(t, parseHashtbl, []parseTest[uint64]{
			// Valid test cases
			{name: "hashtbl-0000000000000001", data: 1, ok: true},
			{name: "hashtbl-ffffffffffffffff", data: 0xffffffffffffffff, ok: true},
			{name: "hashtbl-0000000000000000", data: 0, ok: true},

			// Test cases with capital letters in hex digits (should work - hex is case-insensitive)
			{name: "hashtbl-000000000000000A", data: 0xa, ok: true},
			{name: "hashtbl-000000000000000F", data: 0xf, ok: true},
			{name: "hashtbl-FFFFFFFFFFFFFFFF", data: 0xffffffffffffffff, ok: true},
			{name: "hashtbl-00000000ABCDEF12", data: 0xabcdef12, ok: true},
			{name: "hashtbl-123456789ABCDEF0", data: 0x123456789abcdef0, ok: true},

			// Test cases with invalid hex digits
			{name: "hashtbl-000000000000000g"},

			// Test cases with capital letters in prefix (should NOT match - prefix is case-sensitive)
			{name: "Hashtbl-0000000000000001"},
			{name: "HASHTBL-0000000000000001"},
			{name: "HashTbl-0000000000000001"},
			{name: "hAshtbl-0000000000000001"},

			// Test cases with invalid lengths or prefixes
			{name: "hashtbls-0000000000000001"},
			{name: "hashtbl-000000000000001"},
			{name: "hashtbl-00000000000000001"},
			{name: "log-0000000000000001"},
			{name: "random.txt"},
		})
	})

	t.Run("parseHint", func(t *testing.T) {
		runParseHelperTest(t, parseHint, []parseTest[uint64]{
			// Valid test cases
			{name: "hint-0000000000000001", data: 1, ok: true},
			{name: "hint-ffffffffffffffff", data: 0xffffffffffffffff, ok: true},
			{name: "hint-0000000000000000", data: 0, ok: true},

			// Test cases with capital letters in hex digits (should work - hex is case-insensitive)
			{name: "hint-000000000000000A", data: 0xa, ok: true},
			{name: "hint-000000000000000F", data: 0xf, ok: true},
			{name: "hint-FFFFFFFFFFFFFFFF", data: 0xffffffffffffffff, ok: true},
			{name: "hint-00000000ABCDEF12", data: 0xabcdef12, ok: true},
			{name: "hint-FEDCBA9876543210", data: 0xfedcba9876543210, ok: true},

			// Test cases with invalid hex digits
			{name: "hint-000000000000000g"},

			// Test cases with capital letters in prefix (should NOT match - prefix is case-sensitive)
			{name: "Hint-0000000000000001"},
			{name: "HINT-0000000000000001"},
			{name: "Hint-FFFFFFFFFFFFFFFF"},
			{name: "hInt-0000000000000001"},

			// Test cases with invalid lengths or prefixes
			{name: "hints-0000000000000001"},
			{name: "hint-000000000000001"},
			{name: "hint-00000000000000001"},
			{name: "log-0000000000000001"},
			{name: "random.txt"},
		})
	})

	t.Run("parseTemp", func(t *testing.T) {
		runParseHelperTest(t, parseTemp, []parseTest[string]{
			// Valid test cases
			{name: "file.tmp", data: "file", ok: true},
			{name: "log-0000000000000001.tmp", data: "log-0000000000000001", ok: true},
			{name: "hashtbl-0000000000000001.tmp", data: "hashtbl-0000000000000001", ok: true},
			{name: ".tmp", data: "", ok: true},

			// Test cases with capital letters (should NOT match - suffix is case-sensitive)
			{name: "file.TMP"},
			{name: "file.Tmp"},
			{name: "file.tMp"},
			{name: "LOG-0000000000000001.TMP"},

			// Test cases without .tmp suffix
			{name: "file.txt"},
			{name: "log-0000000000000001"},
			{name: "file.tmp.txt"},
		})
	})
}

func runCreateParseTest[T comparable](t *testing.T, create func(T) string, parse func(string) (T, bool), tests []T) {
	for _, test := range tests {
		name := create(test)
		t.Run(name, func(t *testing.T) {
			parsed, ok := parse(name)
			assert.True(t, ok)
			assert.Equal(t, test, parsed)
		})
	}
}

func TestParseCreatedName(t *testing.T) {
	t.Run("createLogName+parseLog", func(t *testing.T) {
		runCreateParseTest(t, func(info logInfo) string { return createLogName(info.id, info.ttl) }, parseLog, []logInfo{
			{id: 0, ttl: 0},
			{id: 1, ttl: 0},
			{id: 1, ttl: 1},
			{id: 0xffffffffffffffff, ttl: 0},
			{id: 0, ttl: 0xffffffff},
			{id: 0xffffffffffffffff, ttl: 0xffffffff},
			{id: 0x123456789abcdef0, ttl: 0x12345678},
			{id: 42, ttl: 100},
		})
	})

	t.Run("createHashtblName+parseHashtbl", func(t *testing.T) {
		runCreateParseTest(t, createHashtblName, parseHashtbl, []uint64{
			0,
			1,
			0xffffffffffffffff,
			0x123456789abcdef0,
			42,
			100,
		})
	})

	t.Run("createHintName+parseHint", func(t *testing.T) {
		runCreateParseTest(t, createHintName, parseHint, []uint64{
			0,
			1,
			0xffffffffffffffff,
			0x123456789abcdef0,
			42,
			100,
		})
	})

	t.Run("createTempName+parseTemp", func(t *testing.T) {
		runCreateParseTest(t, createTempName, parseTemp, []string{
			"file",
			"log-0000000000000001",
			"hashtbl-0000000000000001",
		})
	})
}

func runParseFilesTest[T any](t *testing.T, dir string, parse func(string) (T, bool), expected map[string]T) {
	var results []parsedFile[T]
	for pf, err := range parseFiles(parse, dir) {
		assert.NoError(t, err)
		results = append(results, pf)
	}

	assert.Equal(t, len(results), len(expected))
	for _, pf := range results {
		expected, ok := expected[pf.name]
		assert.True(t, ok)
		assert.Equal(t, pf.data, expected)
	}
}

func TestParseFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	testFiles := []string{
		"log-0000000000000001",
		"log-0000000000000002-0000ffff",
		"log-0000000000000003-00000000",
		"hashtbl-0000000000000001",
		"hashtbl-0000000000000002",
		"hint-0000000000000001",
		"file.tmp",
		"log-0000000000000004.tmp",
		"random.txt",
		"data.json",
	}

	for _, name := range testFiles {
		touch(t, dir, name)
	}

	// Create subdirectory with files
	subDir := filepath.Join(dir, "subdir")
	assert.NoError(t, os.Mkdir(subDir, 0755))
	touch(t, subDir, "log-0000000000000005")
	touch(t, subDir, "hashtbl-0000000000000003")

	t.Run("parseLog", func(t *testing.T) {
		runParseFilesTest(t, dir, parseLog, map[string]logInfo{
			"log-0000000000000001":          {id: 1, ttl: 0},
			"log-0000000000000002-0000ffff": {id: 2, ttl: 0xffff},
			"log-0000000000000003-00000000": {id: 3, ttl: 0},
			"log-0000000000000005":          {id: 5, ttl: 0},
		})
	})

	t.Run("parseHashtbl", func(t *testing.T) {
		runParseFilesTest(t, dir, parseHashtbl, map[string]uint64{
			"hashtbl-0000000000000001": 1,
			"hashtbl-0000000000000002": 2,
			"hashtbl-0000000000000003": 3,
		})
	})

	t.Run("parseHint", func(t *testing.T) {
		runParseFilesTest(t, dir, parseHint, map[string]uint64{
			"hint-0000000000000001": 1,
		})
	})

	t.Run("parseTemp", func(t *testing.T) {
		runParseFilesTest(t, dir, parseTemp, map[string]string{
			"file.tmp":                 "file",
			"log-0000000000000004.tmp": "log-0000000000000004",
		})
	})

	t.Run("multiple directories", func(t *testing.T) {
		dir2 := t.TempDir()
		touch(t, dir2, "log-0000000000000010")
		touch(t, dir2, "log-0000000000000011")

		count := 0
		for _, err := range parseFiles(parseLog, dir, dir2) {
			assert.NoError(t, err)
			count++
		}
		assert.Equal(t, count, 6)
	})

	t.Run("early termination", func(t *testing.T) {
		count, early := 0, false
		for _, err := range parseFiles(parseLog, dir) {
			assert.NoError(t, err)
			count++
			if count >= 2 {
				early = true
				break
			}
		}
		assert.Equal(t, count, 2)
		assert.True(t, early)
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		called := false
		for _, err := range parseFiles(parseLog, "/nonexistent/directory") {
			assert.Error(t, err)
			called = true
		}
		assert.True(t, called)
	})

	t.Run("empty directory", func(t *testing.T) {
		for range parseFiles(parseLog, t.TempDir()) {
			t.FailNow()
		}
	})
}
