// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	c := Config{
		LogsPath:  "logs",
		TablePath: "tables",
	}
	logs, table := c.Directories("/path/to/storage")
	require.Equal(t, "/path/to/storage/logs", logs)
	require.Equal(t, "/path/to/storage/tables", table)

	c = Config{
		LogsPath:  "/logs",
		TablePath: "/tables",
	}
	logs, table = c.Directories("/path/to/storage")
	require.Equal(t, "/logs", logs)
	require.Equal(t, "/tables", table)
}
