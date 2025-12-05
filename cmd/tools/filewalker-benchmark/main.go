// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/dsnet/try"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/storagenodedb"
)

var (
	numberOfPieces = flag.Int("number-of-pieces", 100000, "")
	lazywalker     = flag.Bool("lazywalker", false, "")

	cpuprofile    = flag.String("cpuprofile", "", "write a cpu profile")
	memprofile    = flag.String("memprofile", "", "write a memory profile")
	dedicatedDisk = flag.Bool("dedicated-disk", false, "assume the test setup is a dedicated disk node. This removes the usage cache layer on top of the blobstore.")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	log := zap.L()

	var cfg storagenode.Config
	setConfigStructDefaults(&cfg)

	snDB := try.E1(storagenodedb.OpenNew(ctx, log, cfg.DatabaseConfig()))
	try.E(snDB.MigrateToLatest(ctx))

	blobStore := snDB.Pieces()
	if !*dedicatedDisk {
		blobStore = pieces.NewBlobsUsageCache(log, snDB.Pieces())
	}
	filewalker := pieces.NewFileWalker(log, blobStore, snDB.V0PieceInfo(),
		snDB.GCFilewalkerProgress(), snDB.UsedSpacePerPrefix())

	piecesStore := pieces.NewStore(log, filewalker, nil, blobStore, snDB.V0PieceInfo(), snDB.PieceExpirationDB(), cfg.Pieces)
	testStore := pieces.StoreForTest{Store: piecesStore}

	satelliteID := testrand.NodeID()

	for i := 0; i < *numberOfPieces; i++ {
		now := time.Now()
		pieceID := testrand.PieceID()
		w := try.E1(testStore.WriterForFormatVersion(ctx, satelliteID, pieceID, filestore.MaxFormatVersionSupported, pb.PieceHashAlgorithm_SHA256))

		try.E1(w.Write(testrand.Bytes(5 * memory.KiB)))

		try.E(w.Commit(ctx, &pb.PieceHeader{
			CreationTime: now,
		}))
	}

	duration := profile("used-space", func() {
		try.E3(filewalker.WalkAndComputeSpaceUsedBySatellite(ctx, satelliteID))
	})
	fmt.Printf("Walked (used space) %d pieces in %s \n", *numberOfPieces, duration)

	filter := bloomfilter.NewOptimal(int64(*numberOfPieces), 0.1)
	trashed := 0
	duration = profile("gc", func() {
		start := time.Now()
		trashFunc := func(pieceID storj.PieceID) error {
			trashed++
			return piecesStore.Trash(ctx, satelliteID, pieceID, start)
		}

		trashHandler := lazyfilewalker.NewTrashHandler(log, trashFunc)
		encoder := json.NewEncoder(trashHandler)
		pieceIDs := make([]storj.PieceID, 0, 1000)

		if *lazywalker {
			trashFunc = func(pieceID storj.PieceID) error {
				pieceIDs = append(pieceIDs, pieceID)

				if len(pieceIDs) >= 1000 {
					resp := lazyfilewalker.GCFilewalkerResponse{
						PieceIDs: pieceIDs,
					}
					pieceIDs = pieceIDs[:0]
					return encoder.Encode(resp)
				}

				return nil
			}
		}

		_, _, err := filewalker.WalkSatellitePiecesToTrash(ctx, satelliteID, start.Add(time.Hour), filter, trashFunc)
		try.E(err)

		if *lazywalker {
			resp := lazyfilewalker.GCFilewalkerResponse{
				PieceIDs: pieceIDs,
			}
			try.E(encoder.Encode(resp))
		}
	})
	fmt.Printf("GC %d pieces in %s, trashed %d\n", *numberOfPieces, duration, trashed)

	duration = profile("trash", func() {
		start := time.Now()
		try.E2(filewalker.WalkCleanupTrash(ctx, satelliteID, start.Add(24*time.Hour)))
	})

	fmt.Printf("Trashed %d pieces in %s\n", trashed, duration)
}

func setConfigStructDefaults(v interface{}) {
	fs := pflag.NewFlagSet("defaults", pflag.ContinueOnError)
	cfgstruct.Bind(fs, v, cfgstruct.ConfDir("."), cfgstruct.IdentityDir("."), cfgstruct.UseReleaseDefaults())
	try.E(fs.Parse(nil))
}

func profile(suffix string, fn func()) time.Duration {
	if *memprofile != "" {
		defer func() {
			memfile := try.E1(os.Create(uniqueProfilePath(*memprofile, suffix)))
			runtime.GC()
			try.E(pprof.WriteHeapProfile(memfile))
			try.E(memfile.Close())
		}()
	}

	if *cpuprofile != "" {
		cpufile := try.E1(os.Create(uniqueProfilePath(*cpuprofile, suffix)))
		defer func() { try.E(cpufile.Close()) }()
		try.E(pprof.StartCPUProfile(cpufile))

		defer func() {
			pprof.StopCPUProfile()
		}()
	}

	start := time.Now()
	fn()
	return time.Since(start)
}

var startTime = time.Now()

func uniqueProfilePath(template, suffix string) string {
	ext := filepath.Ext(template)
	prefix := strings.TrimSuffix(template, ext)

	return prefix + "." + startTime.Format("2006-01-02.15-04-05") + "." + suffix + ext
}
