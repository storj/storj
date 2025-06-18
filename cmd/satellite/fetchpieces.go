// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/process"
	"storj.io/common/uuid"
	"storj.io/common/version"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

func cmdFetchPieces(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	streamID, err := uuid.FromString(args[0])
	if err != nil {
		return errs.New("invalid stream-id (should be in UUID form): %w", err)
	}
	streamPosition, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return errs.New("stream position must be a number: %w", err)
	}
	saveDir := args[2]
	destDirStat, err := os.Stat(saveDir)
	if !destDirStat.IsDir() {
		return errs.New("destination dir %q is not a directory", saveDir)
	}

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Error("Failed to load identity.", zap.Error(err))
		return errs.New("Failed to load identity: %+v", err)
	}

	db, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{ApplicationName: "satellite-pieces-fetcher"})
	if err != nil {
		return errs.New("Error starting master database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL,
		runCfg.Config.Metainfo.Metabase("satellite-pieces-fetcher"))
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	peer, err := satellite.NewRepairer(
		log,
		identity,
		metabaseDB,
		revocationDB,
		nil,
		db.Buckets(),
		db.OverlayCache(),
		db.NodeEvents(),
		db.Reputation(),
		db.Containment(),
		version.Build,
		&runCfg.Config,
		process.AtomicLevel(cmd),
	)
	if err != nil {
		return err
	}

	segmentInfo, err := metabaseDB.GetSegmentByPositionForRepair(ctx, metabase.GetSegmentByPosition{
		StreamID: streamID,
		Position: metabase.SegmentPositionFromEncoded(streamPosition),
	})
	if err != nil {
		return err
	}

	pieceInfos, err := peer.SegmentRepairer.AdminFetchPieces(ctx, log, &segmentInfo, saveDir)
	if err != nil {
		return err
	}

	for pieceIndex, pieceInfo := range pieceInfos {
		if pieceInfo.GetLimit == nil {
			continue
		}
		log := log.With(zap.Int("piece-index", pieceIndex))
		if err := pieceInfo.FetchError; err != nil {
			writeErrorMessageToFile(log, err, saveDir, pieceIndex)
		}
		writeMetaInfoToFile(log, pieceInfo.GetLimit, pieceInfo.OriginalLimit, pieceInfo.Hash, saveDir, pieceIndex)

		if pieceInfo.Reader != nil {
			writePieceToFile(log, pieceInfo.Reader, saveDir, pieceIndex)

			if err := pieceInfo.Reader.Close(); err != nil {
				log.Error("could not close piece reader", zap.Error(err))
			}
		}
	}

	return nil
}

func writeErrorMessageToFile(log *zap.Logger, err error, saveDir string, pieceIndex int) {
	errorMessage := err.Error()
	filename := path.Join(saveDir, fmt.Sprintf("piece.%d.error.txt", pieceIndex))
	log = log.With(zap.String("file-path", filename))
	errorFile, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0o700)
	if err != nil {
		log.Error("could not open error file", zap.Error(err))
		return
	}
	defer func() {
		if err := errorFile.Close(); err != nil {
			log.Error("could not close error file", zap.Error(err))
		}
	}()
	if _, err := errorFile.WriteString(errorMessage); err != nil {
		log.Error("could not write to error file", zap.Error(err))
	}
}

type pieceMetaInfo struct {
	Hash          *pb.PieceHash           `json:"hash"`
	GetLimit      *pb.AddressedOrderLimit `json:"get_limit"`
	OriginalLimit *pb.OrderLimit          `json:"original_limit"`
}

func writeMetaInfoToFile(log *zap.Logger, getLimit *pb.AddressedOrderLimit, originalLimit *pb.OrderLimit, hash *pb.PieceHash, saveDir string, pieceIndex int) {
	filename := path.Join(saveDir, fmt.Sprintf("piece.%d.metainfo.json", pieceIndex))
	log = log.With(zap.String("file-path", filename))
	metaInfoFile, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		log.Error("could not open metainfo file", zap.Error(err))
		return
	}
	defer func() {
		if err := metaInfoFile.Close(); err != nil {
			log.Error("could not close metainfo file", zap.Error(err))
		}
	}()

	metaInfo := pieceMetaInfo{
		Hash:          hash,
		GetLimit:      getLimit,
		OriginalLimit: originalLimit,
	}

	metaInfoJSON, err := json.Marshal(&metaInfo)
	if err != nil {
		log.Error("could not marshal metainfo JSON object", zap.Error(err))
		return
	}
	if _, err := metaInfoFile.Write(metaInfoJSON); err != nil {
		log.Error("could not write JSON to metainfo file", zap.Error(err))
	}
}

func writePieceToFile(log *zap.Logger, pieceReader io.ReadCloser, saveDir string, pieceIndex int) {
	filename := path.Join(saveDir, fmt.Sprintf("piece.%d.contents", pieceIndex))
	log = log.With(zap.String("file-path", filename))
	contentFile, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		log.Error("could not open content file", zap.Error(err))
		return
	}
	defer func() {
		if err := contentFile.Close(); err != nil {
			log.Error("could not close content file", zap.Error(err))
		}
	}()
	if _, err := io.Copy(contentFile, pieceReader); err != nil {
		log.Error("could not copy from piece reader to contents file", zap.Error(err))
	}
}
