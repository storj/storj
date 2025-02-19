// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"path/filepath"
)

// Config is the configuration for the hashstore.
type Config struct {
	LogsPath  string `help:"path to store log files in (by default, it's relative to the storage directory)'" default:"hashstore"`
	TablePath string `help:"path to store tables in. Can be same as LogsPath, as subdirectories are used (by default, it's relative to the storage directory)" default:"hashstore"`
}

// Directories returns the full paths to the logs and tables directories.
func (c Config) Directories(storagePath string) (logsPath string, tablePath string) {
	if filepath.IsAbs(c.LogsPath) {
		logsPath = c.LogsPath
	} else {
		logsPath = filepath.Join(storagePath, c.LogsPath)
	}

	if filepath.IsAbs(c.TablePath) {
		tablePath = c.TablePath
	} else {
		tablePath = filepath.Join(storagePath, c.TablePath)
	}
	return logsPath, tablePath
}
