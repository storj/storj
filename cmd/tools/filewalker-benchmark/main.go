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

	"storj.io/common/bloomfilter"
	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/storagenodedb"
)

var (
	numberOfPieces = flag.Int("number-of-pieces", 100000, "")
	lazywalker     = flag.Bool("lazywalker", false, "")

	cpuprofile = flag.String("cpuprofile", "", "write a cpu profile")
	memprofile = flag.String("memprofile", "", "write a memory profile")
)

func main() {
	flag.Parse()
	ctx := context.Background()
	log := zap.L()

	var cfg storagenode.Config
	setConfigStructDefaults(&cfg)

	snDB := try.E1(storagenodedb.OpenNew(ctx, log, cfg.DatabaseConfig()))
	try.E(snDB.MigrateToLatest(ctx))

	blobsCache := pieces.NewBlobsUsageCache(log, snDB.Pieces())
	filewalker := pieces.NewFileWalker(log, blobsCache, snDB.V0PieceInfo(),
		snDB.GCFilewalkerProgress())

	piecesStore := pieces.NewStore(log, filewalker, nil, blobsCache, snDB.V0PieceInfo(), snDB.PieceExpirationDB(), snDB.PieceSpaceUsedDB(), cfg.Pieces)
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

	filter := bloomfilter.NewOptimal(int64(*numberOfPieces), 0.1)

	start := time.Now()
	trashed := 0
	profile("gc", func() {
		trashFunc := func(pieceID storj.PieceID) error {
			trashed++
			return piecesStore.Trash(ctx, satelliteID, pieceID, start)
		}
		if *lazywalker {
			fmt.Println("using lazy file walker")

			trashHandler := lazyfilewalker.NewTrashHandler(log, trashFunc)

			trashFunc = func(pieceID storj.PieceID) error {
				resp := lazyfilewalker.GCFilewalkerResponse{
					PieceIDs: []storj.PieceID{pieceID},
				}
				return json.NewEncoder(trashHandler).Encode(resp)
			}
		}

		_, _, _, err := filewalker.WalkSatellitePiecesToTrash(ctx, satelliteID, start.Add(time.Hour), filter, trashFunc)
		try.E(err)
	})

	walkDuration := time.Since(start)

	fmt.Printf("GC %d pieces in %s, trashed %d\n", *numberOfPieces, walkDuration, trashed)
}

func setConfigStructDefaults(v interface{}) {
	fs := pflag.NewFlagSet("defaults", pflag.ContinueOnError)
	cfgstruct.Bind(fs, v, cfgstruct.ConfDir("."), cfgstruct.IdentityDir("."), cfgstruct.UseReleaseDefaults())
	try.E(fs.Parse(nil))
}

func profile(suffix string, fn func()) {
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

	fn()
}

var startTime = time.Now()

func uniqueProfilePath(template, suffix string) string {
	ext := filepath.Ext(template)
	prefix := strings.TrimSuffix(template, ext)

	return prefix + "." + startTime.Format("2006-01-02.15-04-05") + "." + suffix + ext
}
