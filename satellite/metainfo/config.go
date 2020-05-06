// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/metainfo/piecedeletion"
	"storj.io/storj/storage"
	"storj.io/storj/storage/cockroachkv"
	"storj.io/storj/storage/postgreskv"
)

const (
	// BoltPointerBucket is the string representing the bucket used for `PointerEntries` in BoltDB
	BoltPointerBucket = "pointers"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     memory.Size `help:"maximum buffer memory to be allocated for read buffers" default:"4MiB"`
	ErasureShareSize memory.Size `help:"the size of each new erasure share in bytes" default:"256B"`
	MinThreshold     int         `help:"the minimum pieces required to recover a segment. k." releaseDefault:"29" devDefault:"4"`
	RepairThreshold  int         `help:"the minimum safe pieces before a repair is triggered. m." releaseDefault:"35" devDefault:"6"`
	SuccessThreshold int         `help:"the desired total pieces for a segment. o." releaseDefault:"80" devDefault:"8"`
	TotalThreshold   int         `help:"the largest amount of pieces to encode to. n." releaseDefault:"110" devDefault:"10"`

	// TODO left for validation until we will remove CreateSegmentOld
	MinTotalThreshold int  `help:"the largest amount of pieces to encode to. n (lower bound for validation)." releaseDefault:"95" devDefault:"10"`
	MaxTotalThreshold int  `help:"the largest amount of pieces to encode to. n (upper bound for validation)." releaseDefault:"130" devDefault:"10"`
	Validate          bool `help:"validate redundancy scheme configuration" default:"true"`
}

// RateLimiterConfig is a configuration struct for endpoint rate limiting
type RateLimiterConfig struct {
	Enabled         bool          `help:"whether rate limiting is enabled." releaseDefault:"true" devDefault:"true"`
	Rate            float64       `help:"request rate per project per second." releaseDefault:"1000" devDefault:"100"`
	CacheCapacity   int           `help:"number of projects to cache." releaseDefault:"10000" devDefault:"10"`
	CacheExpiration time.Duration `help:"how long to cache the projects limiter." releaseDefault:"10m" devDefault:"10s"`
}

// Config is a configuration struct that is everything you need to start a metainfo
type Config struct {
	DatabaseURL          string               `help:"the database connection string to use" default:"postgres://"`
	MinRemoteSegmentSize memory.Size          `default:"1240" help:"minimum remote segment size"`
	MaxInlineSegmentSize memory.Size          `default:"4KiB" help:"maximum inline segment size"`
	MaxSegmentSize       memory.Size          `default:"64MiB" help:"maximum segment size"`
	MaxCommitInterval    time.Duration        `default:"48h" help:"maximum time allowed to pass between creating and committing a segment"`
	Overlay              bool                 `default:"true" help:"toggle flag if overlay is enabled"`
	RS                   RSConfig             `help:"redundancy scheme configuration"`
	Loop                 LoopConfig           `help:"loop configuration"`
	RateLimiter          RateLimiterConfig    `help:"rate limiter configuration"`
	PieceDeletion        piecedeletion.Config `help:"piece deletion configuration"`
}

// PointerDB stores pointers.
//
// architecture: Database
type PointerDB interface {
	// MigrateToLatest migrates to latest schema version.
	MigrateToLatest(ctx context.Context) error

	storage.KeyValueStore
}

// NewStore returns database for storing pointer data.
func NewStore(logger *zap.Logger, dbURLString string) (db PointerDB, err error) {
	_, source, implementation, err := dbutil.SplitConnStr(dbURLString)
	if err != nil {
		return nil, err
	}

	switch implementation {
	case dbutil.Postgres:
		db, err = postgreskv.New(source)
	case dbutil.Cockroach:
		db, err = cockroachkv.New(source)
	default:
		err = Error.New("unsupported db implementation: %s", dbURLString)
	}

	if err != nil {
		return nil, err
	}

	logger.Debug("Connected to:", zap.String("db source", source))
	return db, nil
}
