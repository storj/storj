// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information

package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/dbutil/cockroachutil"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/tagsql"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/storagenode/pieces"
)

func TestCommandLineTool(t *testing.T) {
	const (
		nodeCount   = 10
		uplinkCount = 10
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: nodeCount, UplinkCount: uplinkCount,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(nodeCount, nodeCount, nodeCount, nodeCount),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		// get the db connstrings that we can set in the global config (these are hilariously hard to get,
		// but we really don't need to get them anywhere else in the codebase)
		dbConnString := getConnStringFromDBConn(t, ctx, satellite.DB.Testing().RawDB())
		metaDBConnString := getConnStringFromDBConn(t, ctx, satellite.Metabase.DB.UnderlyingTagSQL())

		notFoundCSV := ctx.File("notfound.csv")
		retryCSV := ctx.File("retry.csv")
		problemPiecesCSV := ctx.File("problempieces.csv")

		// set up global config that the main func will use
		satelliteCfg := satelliteCfg
		satelliteCfg.Config = satellite.Config
		satelliteCfg.Database = dbConnString
		satelliteCfg.Metainfo.DatabaseURL = metaDBConnString
		satelliteCfg.Identity.KeyPath = ctx.File("identity-key")
		satelliteCfg.Identity.CertPath = ctx.File("identity-cert")
		require.NoError(t, satelliteCfg.Identity.Save(satellite.Identity))
		rangeCfg := rangeCfg
		rangeCfg.Verify = VerifierConfig{
			PerPieceTimeout:    time.Second,
			OrderRetryThrottle: 500 * time.Millisecond,
			RequestThrottle:    500 * time.Millisecond,
		}
		rangeCfg.Service = ServiceConfig{
			NotFoundPath:           notFoundCSV,
			RetryPath:              retryCSV,
			ProblemPiecesPath:      problemPiecesCSV,
			Check:                  0,
			BatchSize:              10000,
			Concurrency:            1000,
			MaxOffline:             2,
			OfflineStatusCacheTime: 10 * time.Second,
			AsOfSystemInterval:     -1 * time.Microsecond,
		}
		rangeCfg.Low = strings.Repeat("0", 32)
		rangeCfg.High = strings.Repeat("f", 32)

		// upload some data
		data := testrand.Bytes(8 * memory.KiB)
		for u, up := range planet.Uplinks {
			for i := 0; i < nodeCount; i++ {
				err := up.Upload(ctx, satellite, "bucket1", fmt.Sprintf("uplink%d/i%d", u, i), data)
				require.NoError(t, err)
			}
		}

		// take one node offline so there will be some pieces in the retry list
		offlineNode := planet.StorageNodes[0]
		require.NoError(t, planet.StopPeer(offlineNode))

		// and delete 10% of pieces at random so there will be some pieces in the not-found list
		const deleteFrac = 0.10
		allDeletedPieces := make(map[storj.NodeID]map[storj.PieceID]struct{})
		numDeletedPieces := 0
		for nodeNum, node := range planet.StorageNodes {
			if node.ID() == offlineNode.ID() {
				continue
			}
			deletedPieces, err := deletePiecesRandomly(ctx, satellite.ID(), node, deleteFrac)
			require.NoError(t, err, nodeNum)
			allDeletedPieces[node.ID()] = deletedPieces
			numDeletedPieces += len(deletedPieces)
		}

		// check that the number of segments we expect are present in the metainfo db
		result, err := satellite.Metabase.DB.ListVerifySegments(ctx, metabase.ListVerifySegments{
			CursorStreamID: uuid.UUID{},
			CursorPosition: metabase.SegmentPosition{},
			Limit:          10000,
		})
		require.NoError(t, err)
		require.Len(t, result.Segments, uplinkCount*nodeCount)

		// perform the verify!
		log := zaptest.NewLogger(t)
		err = verifySegmentsInContext(ctx, log, &cobra.Command{Use: "range"}, satelliteCfg, rangeCfg)
		require.NoError(t, err)

		// open the CSVs to check that we get the expected results
		retryCSVHandle, err := os.Open(retryCSV)
		require.NoError(t, err)
		defer ctx.Check(retryCSVHandle.Close)
		retryCSVReader := csv.NewReader(retryCSVHandle)

		notFoundCSVHandle, err := os.Open(notFoundCSV)
		require.NoError(t, err)
		defer ctx.Check(notFoundCSVHandle.Close)
		notFoundCSVReader := csv.NewReader(notFoundCSVHandle)

		problemPiecesCSVHandle, err := os.Open(problemPiecesCSV)
		require.NoError(t, err)
		defer ctx.Check(problemPiecesCSVHandle.Close)
		problemPiecesCSVReader := csv.NewReader(problemPiecesCSVHandle)

		// in the retry CSV, we don't expect any rows, because there would need to be more than 5
		// nodes offline to produce records here.
		// TODO: make that 5 configurable so we can override it here and check results
		header, err := retryCSVReader.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"stream id", "position", "found", "not found", "retry"}, header)
		for {
			record, err := retryCSVReader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			assert.Fail(t, "unexpected record in retry.csv", "%v", record)
		}

		// we do expect plenty of rows in not-found.csv. we don't know exactly what pieces these
		// pertain to, but we can add up all the reported not-found pieces and expect the total
		// to match numDeletedPieces. In addition, for each segment, found+notfound+retry should
		// equal nodeCount.
		header, err = notFoundCSVReader.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"stream id", "position", "found", "not found", "retry"}, header)
		identifiedNotFoundPieces := 0
		for {
			record, err := notFoundCSVReader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			found, err := strconv.Atoi(record[2])
			require.NoError(t, err)
			notFound, err := strconv.Atoi(record[3])
			require.NoError(t, err)
			retry, err := strconv.Atoi(record[4])
			require.NoError(t, err)

			lineNum, _ := notFoundCSVReader.FieldPos(0)
			assert.Equal(t, nodeCount, found+notFound+retry,
				"line %d of not-found.csv contains record: %v where found+notFound+retry != %d", lineNum, record, nodeCount)
			identifiedNotFoundPieces += notFound
		}
		assert.Equal(t, numDeletedPieces, identifiedNotFoundPieces)

		// finally, in problem-pieces.csv, we can check results with more precision. we expect
		// that all deleted pieces were identified, and that no pieces were identified as not found
		// unless we deleted them specifically.
		header, err = problemPiecesCSVReader.Read()
		require.NoError(t, err)
		assert.Equal(t, []string{"stream id", "position", "node id", "piece number", "outcome"}, header)
		for {
			record, err := problemPiecesCSVReader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			streamID, err := uuid.FromString(record[0])
			require.NoError(t, err)
			position, err := strconv.ParseUint(record[1], 10, 64)
			require.NoError(t, err)
			nodeID, err := storj.NodeIDFromString(record[2])
			require.NoError(t, err)
			pieceNum, err := strconv.ParseInt(record[3], 10, 16)
			require.NoError(t, err)
			outcome := record[4]

			switch outcome {
			case "NODE_OFFLINE":
				// expect that this was the node we took offline
				assert.Equal(t, offlineNode.ID(), nodeID,
					"record %v said node %s was offline, but we didn't take it offline", record, nodeID)
			case "NOT_FOUND":
				segmentPosition := metabase.SegmentPositionFromEncoded(position)
				segment, err := satellite.Metabase.DB.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
					StreamID: streamID,
					Position: segmentPosition,
				})
				require.NoError(t, err)
				pieceID := segment.RootPieceID.Derive(nodeID, int32(pieceNum))

				deletedPiecesForNode, ok := allDeletedPieces[nodeID]
				require.True(t, ok)
				_, ok = deletedPiecesForNode[pieceID]
				assert.True(t, ok, "we did not delete piece ID %s, but it was identified as not found", pieceID)
				delete(deletedPiecesForNode, pieceID)
			default:
				assert.Fail(t, "unexpected outcome from problem-pieces.csv", "got %q, but expected \"NODE_OFFLINE\" or \"NOT_FOUND\"", outcome)
			}
		}

		for node, deletedPieces := range allDeletedPieces {
			assert.Empty(t, deletedPieces, "pieces were deleted from %v but were not reported in problem-pieces.csv", node)
		}
	})
}

func deletePiecesRandomly(ctx context.Context, satelliteID storj.NodeID, node *testplanet.StorageNode, rate float64) (deletedPieces map[storj.PieceID]struct{}, err error) {
	deletedPieces = make(map[storj.PieceID]struct{})
	err = node.Storage2.FileWalker.WalkSatellitePieces(ctx, satelliteID, func(access pieces.StoredPieceAccess) error {
		if rand.Float64() < rate {
			path, err := access.FullPath(ctx)
			if err != nil {
				return err
			}
			err = os.Remove(path)
			if err != nil {
				return err
			}
			deletedPieces[access.PieceID()] = struct{}{}
		}
		return nil
	})
	return deletedPieces, err
}

func getConnStringFromDBConn(t *testing.T, ctx *testcontext.Context, tagsqlDB tagsql.DB) (dbConnString string) {
	type dbConnGetter interface {
		StdlibConn() *stdlib.Conn
	}

	dbConn, err := tagsqlDB.Conn(ctx)
	require.NoError(t, err)
	defer ctx.Check(dbConn.Close)
	err = dbConn.Raw(ctx, func(driverConn interface{}) error {
		var stdlibConn *stdlib.Conn
		switch conn := driverConn.(type) {
		case dbConnGetter:
			stdlibConn = conn.StdlibConn()
		case *stdlib.Conn:
			stdlibConn = conn
		}
		dbConnString = stdlibConn.Conn().Config().ConnString()
		return nil
	})
	require.NoError(t, err)
	if _, ok := tagsqlDB.Driver().(*cockroachutil.Driver); ok {
		dbConnString = strings.ReplaceAll(dbConnString, "postgres://", "cockroach://")
	}
	return dbConnString
}
