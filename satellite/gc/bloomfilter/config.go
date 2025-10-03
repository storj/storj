// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"time"

	"storj.io/common/memory"
	"storj.io/common/storj"
)

// Config contains configurable values for garbage collection.
type Config struct {
	RunOnce bool `help:"set if garbage collection bloom filter process should only run once then exit" default:"false"`

	UseSyncObserver   bool `help:"whether to use test GC SyncObserver with ranged loop" default:"true"`
	UseSyncObserverV2 bool `help:"whether to use SyncObserverV2 for GC" default:"false"`

	// value for InitialPieces currently based on average pieces per node
	InitialPieces        int64       `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate    float64     `help:"the false positive rate used for creating a garbage collection bloom filter" releaseDefault:"0.1" devDefault:"0.1"`
	MaxBloomFilterSize   memory.Size `help:"maximum size of a single bloom filter" default:"2m"`
	ExcludeExpiredPieces bool        `help:"do not include expired pieces into bloom filter" default:"true"`

	AccessGrant  string        `help:"Access Grant which will be used to upload bloom filters to the bucket" default:""`
	Bucket       string        `help:"Bucket which will be used to upload bloom filters" default:"" testDefault:"gc-queue"` // TODO do we need full location?
	ZipBatchSize int           `help:"how many bloom filters will be packed in a single zip" default:"40" testDefault:"2"`
	ExpireIn     time.Duration `help:"how long bloom filters will remain in the bucket for gc/sender to consume before being automatically deleted" default:"336h"`

	CollectNodesPieceIDs    storj.NodeIDList `help:"list of node IDs for which we will collect raw list of piece IDs" default:""`
	NodesPieceIDsBufferSize int              `help:"buffer size of piece IDs before will be uploaded" default:"1000000" devDefault:"100"`
	UploadPackConcurrency   int              `help:"number of concurrent zip compression and uploads of bloom filters" default:"4"`
}
