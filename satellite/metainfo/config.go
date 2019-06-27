// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"go.uber.org/zap"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/internal/memory"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/postgreskv"
)

const (
	// BoltPointerBucket is the string representing the bucket used for `PointerEntries` in BoltDB
	BoltPointerBucket = "pointers"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information
type RSConfig struct {
	MaxBufferMem     memory.Size `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4MiB"`
	ErasureShareSize memory.Size `help:"the size of each new erasure share in bytes" default:"256B"`
	MinThreshold     int         `help:"the minimum pieces required to recover a segment. k." releaseDefault:"29" devDefault:"4"`
	RepairThreshold  int         `help:"the minimum safe pieces before a repair is triggered. m." releaseDefault:"35" devDefault:"6"`
	SuccessThreshold int         `help:"the desired total pieces for a segment. o." releaseDefault:"80" devDefault:"8"`
	MaxThreshold     int         `help:"the largest amount of pieces to encode to. n." releaseDefault:"130" devDefault:"10"`
	Validate         bool        `help:"validate redundancy scheme configuration" default:"true"`
}

// Config is a configuration struct that is everything you need to start a metainfo
type Config struct {
	DatabaseURL          string      `help:"the database connection string to use" releaseDefault:"postgres://" devDefault:"bolt://$CONFDIR/pointerdb.db"`
	MinRemoteSegmentSize memory.Size `default:"1240" help:"minimum remote segment size"`
	MaxInlineSegmentSize memory.Size `default:"8000" help:"maximum inline segment size"`
	Overlay              bool        `default:"true" help:"toggle flag if overlay is enabled"`
	BwExpiration         int         `default:"45"   help:"lifespan of bandwidth agreements in days"`
	RS                   RSConfig    `help:"redundancy scheme configuration"`
}

// NewStore returns database for storing pointer data
func NewStore(logger *zap.Logger, dbURLString string) (db storage.KeyValueStore, err error) {
	driver, source, err := dbutil.SplitConnstr(dbURLString)
	if err != nil {
		return nil, err
	}

	switch driver {
	case "bolt":
		db, err = boltdb.New(source, BoltPointerBucket)
	case "postgresql", "postgres":
		db, err = postgreskv.New(source)
	default:
		err = Error.New("unsupported db scheme: %s", driver)
	}

	logger.Debug("Connected to:", zap.String("db source", source))
	return db, err
}
