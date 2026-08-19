// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package bloomfilter

import (
	"time"

	"storj.io/common/memory"
)

// Config contains configurable values for garbage collection.
type Config struct {
	RunOnce bool `help:"set if garbage collection bloom filter process should only run once then exit" default:"false"`

	// value for InitialPieces currently based on average pieces per node
	InitialPieces      int64       `help:"the initial number of pieces expected for a storage node to have, used for creating a filter" releaseDefault:"400000" devDefault:"10"`
	FalsePositiveRate  float64     `help:"the false positive rate used for creating a garbage collection bloom filter" releaseDefault:"0.1" devDefault:"0.1"`
	MaxBloomFilterSize memory.Size `help:"maximum size of a single bloom filter" default:"2m"`

	AccessGrant  string        `help:"Access Grant which will be used to upload bloom filters to the bucket" default:""`
	Bucket       string        `help:"Bucket which will be used to upload bloom filters" default:"" testDefault:"gc-queue"` // TODO do we need full location?
	ZipBatchSize int           `help:"how many bloom filters will be packed in a single zip" default:"40" testDefault:"2"`
	ExpireIn     time.Duration `help:"how long bloom filters will remain in the bucket for gc/sender to consume before being automatically deleted" default:"336h"`

	UploadPackConcurrency int `help:"number of concurrent zip compression and uploads of bloom filters" default:"4"`

	ShardCount   int    `help:"number of node shards; each pass builds bloom filters for 1/N of the nodes to reduce memory. Requires run-once mode, since one job runs every shard. All passes read the same database snapshot, so ranged-loop.safepoint.max-duration and the backing store's GC TTL must be at least N times a single scan" default:"1"`
	Shard        int    `help:"run only this shard (0-based); -1 runs all shards sequentially" default:"-1"`
	UploadPrefix string `help:"object prefix under the bucket to upload into; empty generates a timestamp prefix. Set to the previous run's LATEST value when rerunning a single shard" default:""`
}
