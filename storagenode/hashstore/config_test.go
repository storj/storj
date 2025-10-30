// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zeebo/assert"
)

func TestTableKindCfg(t *testing.T) {
	var k TableKindCfg

	assert.Equal(t, k.Type(), "TableKind")

	assert.NoError(t, k.Set(""))
	assert.Equal(t, k.Kind, TableKind_HashTbl)
	assert.Equal(t, k.String(), "HashTbl")

	assert.NoError(t, k.Set("hashtbl"))
	assert.Equal(t, k.Kind, TableKind_HashTbl)
	assert.Equal(t, k.String(), "HashTbl")

	assert.NoError(t, k.Set("hash"))
	assert.Equal(t, k.Kind, TableKind_HashTbl)
	assert.Equal(t, k.String(), "HashTbl")

	assert.NoError(t, k.Set("memtbl"))
	assert.Equal(t, k.Kind, TableKind_MemTbl)
	assert.Equal(t, k.String(), "MemTbl")

	assert.NoError(t, k.Set("mem"))
	assert.Equal(t, k.Kind, TableKind_MemTbl)
	assert.Equal(t, k.String(), "MemTbl")

	assert.Error(t, k.Set("unknown"))
}

func TestConfig(t *testing.T) {
	baseDir := filepath.FromSlash("/path/to/storage")

	t.Run("relative dirs", func(t *testing.T) {
		c := Config{
			LogsPath:  "logs",
			TablePath: "tables",
		}
		logs, table := c.Directories(baseDir)
		assert.Equal(t, filepath.Join(baseDir, "logs"), logs)
		assert.Equal(t, filepath.Join(baseDir, "tables"), table)

	})

	t.Run("absolute logs path", func(t *testing.T) {
		pwd, err := os.Getwd()
		assert.NoError(t, err)

		absLogs, err := filepath.Abs(filepath.Join(pwd, "logs"))
		assert.NoError(t, err)

		absTables, err := filepath.Abs(filepath.Join(pwd, "tables"))
		assert.NoError(t, err)

		c := Config{
			LogsPath:  absLogs,
			TablePath: absTables,
		}
		logs, table := c.Directories(baseDir)
		assert.Equal(t, absLogs, logs)
		assert.Equal(t, absTables, table)
	})
}
