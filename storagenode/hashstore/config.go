// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

// Config is the configuration for the hashstore.
type Config struct {
	SyncLifo         bool         `help:"controls if waiters are processed in LIFO or FIFO order." default:"false" hidden:"true"`
	LogsPath         string       `help:"path to store log files in (by default, it's relative to the storage directory)'" default:"hashstore"`
	TablePath        string       `help:"path to store tables in. Can be same as LogsPath, as subdirectories are used (by default, it's relative to the storage directory)" default:"hashstore"`
	TableDefaultKind TableKindCfg `help:"default table kind to use (hashtbl or memtbl) during NEW compations" default:"hashtbl"`
	Store            StoreCfg
	Compaction       CompactionCfg
	Hashtbl          MmapCfg
	Memtbl           MmapCfg
}

// TableKindCfg is a wrapper around TableKind to implement flag.Value.
type TableKindCfg struct {
	Kind TableKind
}

// Type returns the type of the table kind (implements flag.Value).
func (t *TableKindCfg) Type() string {
	return "TableKind"
}

// Set sets the table kind from a string (implements flag.Value).
func (t *TableKindCfg) Set(s string) error {
	// ParseTableKind returns a TableKind for the given string.
	switch strings.ToLower(s) {
	case "", "hashtbl", "hash":
		t.Kind = TableKind_HashTbl
		return nil
	case "memtbl", "mem":
		t.Kind = TableKind_MemTbl
		return nil
	default:
		return fmt.Errorf("unknown table kind: %q", s)
	}
}

// String returns a string representation of the table kind (implements flag.Value).
func (t *TableKindCfg) String() string {
	return t.Kind.String()
}

var _ pflag.Value = (*TableKindCfg)(nil)

// CompactionCfg is the configuration for log compaction.
type CompactionCfg struct {
	MaxLogSize             uint64  `help:"max size of a log file" default:"1073741824"`
	ExpiresDays            uint64  `help:"number of days to keep trash records around" default:"7" hidden:"true"`
	AliveFraction          float64 `help:"if the log file is not this alive, compact it" default:"0.25"`
	ProbabilityPower       float64 `help:"power to raise the rewrite probability to. >1 means must be closer to the alive fraction to be compacted, <1 means the opposite" default:"2.0"`
	RewriteMultiple        float64 `help:"multiple of the hashtbl to rewrite in a single compaction" default:"10"`
	DeleteTrashImmediately bool    `help:"if set, deletes all trash immediately instead of after the ttl" default:"false" hidden:"true"`
	OrderedRewrite         bool    `help:"controls if we collect records and sort them and rewrite them before the hashtbl" default:"true"`
}

// StoreCfg is the configuration for the store.
type StoreCfg struct {
	FlushSemaphore int  `help:"controls the number of concurrent flushes to log files" default:"0" hidden:"true"`
	SyncWrites     bool `help:"if set, writes to the log file and table are fsync'd to disk" default:"false"`
	OpenFileCache  int  `help:"number of open file handles to cache for reads" default:"10"`
}

// MmapCfg is the configuration for mmap usage.
type MmapCfg struct {
	Mmap  bool `help:"if set, uses mmap to do reads" default:"false"`
	Mlock bool `help:"if set, call mlock on any mmap/mremap'd data" default:"true"`
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

// CreateDefaultConfig returns a default configuration suitable for testing and cli utilities.
// kind and mmap control the table kind and whether to use mmap for reads.
func CreateDefaultConfig(kind TableKind, mmap bool) Config {
	return Config{
		SyncLifo:  false,
		LogsPath:  "hashstore",
		TablePath: "hashstore",
		TableDefaultKind: TableKindCfg{
			Kind: kind,
		},
		Store: StoreCfg{
			FlushSemaphore: 0,
			SyncWrites:     false,
			OpenFileCache:  10,
		},
		Compaction: CompactionCfg{
			MaxLogSize:             1073741824,
			ExpiresDays:            7,
			AliveFraction:          0.25,
			ProbabilityPower:       2.0,
			RewriteMultiple:        10,
			DeleteTrashImmediately: false,
			OrderedRewrite:         true,
		},
		Hashtbl: MmapCfg{
			Mmap:  mmap,
			Mlock: true,
		},
		Memtbl: MmapCfg{
			Mmap:  mmap,
			Mlock: true,
		},
	}
}
