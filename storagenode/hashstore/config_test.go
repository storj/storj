// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {

	baseDir := filepath.FromSlash("/path/to/storage")

	t.Run("relative dirs", func(t *testing.T) {
		c := Config{
			LogsPath:  "logs",
			TablePath: "tables",
		}
		logs, table := c.Directories(baseDir)
		require.Equal(t, filepath.Join(baseDir, "logs"), logs)
		require.Equal(t, filepath.Join(baseDir, "tables"), table)

	})

	t.Run("absolute logs path", func(t *testing.T) {
		pwd, err := os.Getwd()
		require.NoError(t, err)

		absLogs, err := filepath.Abs(filepath.Join(pwd, "logs"))
		require.NoError(t, err)

		absTables, err := filepath.Abs(filepath.Join(pwd, "tables"))
		require.NoError(t, err)

		c := Config{
			LogsPath:  absLogs,
			TablePath: absTables,
		}
		logs, table := c.Directories(baseDir)
		require.Equal(t, absLogs, logs)
		require.Equal(t, absTables, table)
	})
}
