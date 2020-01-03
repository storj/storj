// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity/testidentity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/pkcrypto"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/trust"
)

func TestPieces(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dir, err := filestore.NewDir(ctx.Dir("pieces"))
	require.NoError(t, err)

	blobs := filestore.New(zaptest.NewLogger(t), dir)
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zaptest.NewLogger(t), blobs, nil, nil, nil)

	satelliteID := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion()).ID
	pieceID := storj.NewPieceID()

	source := testrand.Bytes(8000)

	{ // write data
		writer, err := store.Writer(ctx, satelliteID, pieceID)
		require.NoError(t, err)

		n, err := io.Copy(writer, bytes.NewReader(source))
		require.NoError(t, err)
		assert.Equal(t, len(source), int(n))
		assert.Equal(t, len(source), int(writer.Size()))

		// verify hash
		hash := pkcrypto.NewHash()
		_, _ = hash.Write(source)
		assert.Equal(t, hash.Sum(nil), writer.Hash())

		// commit
		require.NoError(t, writer.Commit(ctx, &pb.PieceHeader{}))
		// after commit we should be able to call cancel without an error
		require.NoError(t, writer.Cancel(ctx))
	}

	{ // valid reads
		read := func(offset, length int64) []byte {
			reader, err := store.Reader(ctx, satelliteID, pieceID)
			require.NoError(t, err)

			pos, err := reader.Seek(offset, io.SeekStart)
			require.NoError(t, err)
			require.Equal(t, offset, pos)

			data := make([]byte, length)
			n, err := io.ReadFull(reader, data)
			require.NoError(t, err)
			require.Equal(t, int(length), n)

			require.NoError(t, reader.Close())

			return data
		}

		require.Equal(t, source[10:11], read(10, 1))
		require.Equal(t, source[10:1010], read(10, 1000))
		require.Equal(t, source, read(0, int64(len(source))))
	}

	{ // reading ends with io.EOF
		reader, err := store.Reader(ctx, satelliteID, pieceID)
		require.NoError(t, err)

		data := make([]byte, 111)
		for {
			_, err := reader.Read(data)
			if err != nil {
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
		}

		require.NoError(t, reader.Close())
	}

	{ // test delete
		assert.NoError(t, store.Delete(ctx, satelliteID, pieceID))
		// read should now fail
		_, err := store.Reader(ctx, satelliteID, pieceID)
		assert.Error(t, err)
	}

	{ // write cancel
		cancelledPieceID := storj.NewPieceID()
		writer, err := store.Writer(ctx, satelliteID, cancelledPieceID)
		require.NoError(t, err)

		n, err := io.Copy(writer, bytes.NewReader(source))
		require.NoError(t, err)
		assert.Equal(t, len(source), int(n))
		assert.Equal(t, len(source), int(writer.Size()))

		// cancel writing
		require.NoError(t, writer.Cancel(ctx))
		// commit should not fail
		require.Error(t, writer.Commit(ctx, &pb.PieceHeader{}))

		// read should fail
		_, err = store.Reader(ctx, satelliteID, cancelledPieceID)
		assert.Error(t, err)
	}
}

func writeAPiece(ctx context.Context, t testing.TB, store *pieces.Store, satelliteID storj.NodeID, pieceID storj.PieceID, data []byte, atTime time.Time, expireTime *time.Time, formatVersion storage.FormatVersion) {
	tStore := &pieces.StoreForTest{store}
	writer, err := tStore.WriterForFormatVersion(ctx, satelliteID, pieceID, formatVersion)
	require.NoError(t, err)

	_, err = writer.Write(data)
	require.NoError(t, err)
	size := writer.Size()
	assert.Equal(t, int64(len(data)), size)
	limit := pb.OrderLimit{}
	if expireTime != nil {
		limit.PieceExpiration = *expireTime
	}
	err = writer.Commit(ctx, &pb.PieceHeader{
		Hash:         writer.Hash(),
		CreationTime: atTime,
		OrderLimit:   limit,
	})
	require.NoError(t, err)
}

func verifyPieceHandle(t testing.TB, reader *pieces.Reader, expectDataLen int, expectCreateTime time.Time, expectFormat storage.FormatVersion) {
	assert.Equal(t, expectFormat, reader.StorageFormatVersion())
	assert.Equal(t, int64(expectDataLen), reader.Size())
	if expectFormat != filestore.FormatV0 {
		pieceHeader, err := reader.GetPieceHeader()
		require.NoError(t, err)
		assert.Equal(t, expectFormat, storage.FormatVersion(pieceHeader.FormatVersion))
		assert.Equal(t, expectCreateTime.UTC(), pieceHeader.CreationTime.UTC())
	}
}

func tryOpeningAPiece(ctx context.Context, t testing.TB, store *pieces.Store, satelliteID storj.NodeID, pieceID storj.PieceID, expectDataLen int, expectTime time.Time, expectFormat storage.FormatVersion) {
	reader, err := store.Reader(ctx, satelliteID, pieceID)
	require.NoError(t, err)
	verifyPieceHandle(t, reader, expectDataLen, expectTime, expectFormat)
	require.NoError(t, reader.Close())

	reader, err = store.ReaderWithStorageFormat(ctx, satelliteID, pieceID, expectFormat)
	require.NoError(t, err)
	verifyPieceHandle(t, reader, expectDataLen, expectTime, expectFormat)
	require.NoError(t, reader.Close())
}

func TestTrashAndRestore(t *testing.T) {
	type testfile struct {
		data      []byte
		formatVer storage.FormatVersion
	}
	type testpiece struct {
		pieceID    storj.PieceID
		files      []testfile
		expiration time.Time
		trashDur   time.Duration
	}
	type testsatellite struct {
		satelliteID storj.NodeID
		pieces      []testpiece
	}

	size := memory.KB

	// Initialize pub/priv keys for signing piece hash
	publicKeyBytes, err := hex.DecodeString("01eaebcb418cd629d4c01f365f33006c9de3ce70cf04da76c39cdc993f48fe53")
	require.NoError(t, err)
	privateKeyBytes, err := hex.DecodeString("afefcccadb3d17b1f241b7c83f88c088b54c01b5a25409c13cbeca6bfa22b06901eaebcb418cd629d4c01f365f33006c9de3ce70cf04da76c39cdc993f48fe53")
	require.NoError(t, err)
	publicKey, err := storj.PiecePublicKeyFromBytes(publicKeyBytes)
	require.NoError(t, err)
	privateKey, err := storj.PiecePrivateKeyFromBytes(privateKeyBytes)
	require.NoError(t, err)

	trashDurToBeEmptied := 7 * 24 * time.Hour
	trashDurToBeKept := 3 * 24 * time.Hour
	satellites := []testsatellite{
		{
			satelliteID: testrand.NodeID(),
			pieces: []testpiece{
				{
					expiration: time.Time{}, // no expiration
					pieceID:    testrand.PieceID(),
					trashDur:   trashDurToBeEmptied,
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
					},
				},
				{
					pieceID:    testrand.PieceID(),
					expiration: time.Now().Add(24 * time.Hour),
					trashDur:   trashDurToBeEmptied,
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
				{
					pieceID:    testrand.PieceID(),
					expiration: time.Now().Add(24 * time.Hour),
					trashDur:   trashDurToBeKept,
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
				{
					pieceID:    testrand.PieceID(),
					expiration: time.Now().Add(24 * time.Hour),
					trashDur:   trashDurToBeKept,
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
				{
					pieceID:    testrand.PieceID(),
					expiration: time.Now().Add(24 * time.Hour),
					trashDur:   trashDurToBeKept,
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV0,
						},
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
			},
		},
		{
			satelliteID: testrand.NodeID(),
			pieces: []testpiece{
				{
					pieceID:    testrand.PieceID(),
					expiration: time.Now().Add(24 * time.Hour),
					trashDur:   trashDurToBeKept,
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
				{
					pieceID:    testrand.PieceID(),
					expiration: time.Now().Add(24 * time.Hour),
					trashDur:   trashDurToBeEmptied,
					files: []testfile{
						{
							data:      testrand.Bytes(size),
							formatVer: filestore.FormatV1,
						},
					},
				},
			},
		},
	}

	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		dir, err := filestore.NewDir(ctx.Dir("store"))
		require.NoError(t, err)

		blobs := filestore.New(zaptest.NewLogger(t), dir)
		require.NoError(t, err)
		defer ctx.Check(blobs.Close)

		v0PieceInfo, ok := db.V0PieceInfo().(pieces.V0PieceInfoDBForTest)
		require.True(t, ok, "V0PieceInfoDB can not satisfy V0PieceInfoDBForTest")

		store := pieces.NewStore(zaptest.NewLogger(t), blobs, v0PieceInfo, db.PieceExpirationDB(), nil)
		tStore := &pieces.StoreForTest{store}

		var satelliteURLs []trust.SatelliteURL
		for i, satellite := range satellites {
			// host:port pair must be unique or the trust pool will aggregate
			// them into a single entry with the first one "winning".
			satelliteURLs = append(satelliteURLs, trust.SatelliteURL{
				ID:   satellite.satelliteID,
				Host: "localhost",
				Port: i,
			})
			now := time.Now()
			for _, piece := range satellite.pieces {
				// If test has expiration, add to expiration db
				if !piece.expiration.IsZero() {
					require.NoError(t, store.SetExpiration(ctx, satellite.satelliteID, piece.pieceID, piece.expiration))
				}

				for _, file := range piece.files {
					w, err := tStore.WriterForFormatVersion(ctx, satellite.satelliteID, piece.pieceID, file.formatVer)
					require.NoError(t, err)

					_, err = w.Write(file.data)
					require.NoError(t, err)

					// Create, sign, and commit piece hash (to piece or v0PieceInfo)
					pieceHash := &pb.PieceHash{
						PieceId:   piece.pieceID,
						Hash:      w.Hash(),
						PieceSize: w.Size(),
						Timestamp: now,
					}
					signedPieceHash, err := signing.SignUplinkPieceHash(ctx, privateKey, pieceHash)
					require.NoError(t, err)
					require.NoError(t, w.Commit(ctx, &pb.PieceHeader{
						Hash:         signedPieceHash.GetHash(),
						CreationTime: signedPieceHash.GetTimestamp(),
						Signature:    signedPieceHash.GetSignature(),
					}))

					if file.formatVer == filestore.FormatV0 {
						err = v0PieceInfo.Add(ctx, &pieces.Info{
							SatelliteID:     satellite.satelliteID,
							PieceID:         piece.pieceID,
							PieceSize:       signedPieceHash.GetPieceSize(),
							PieceCreation:   signedPieceHash.GetTimestamp(),
							OrderLimit:      &pb.OrderLimit{},
							UplinkPieceHash: signedPieceHash,
						})
						require.NoError(t, err)
					}

					// Verify piece matches data, has correct signature and expiration
					verifyPieceData(ctx, t, store, satellite.satelliteID, piece.pieceID, file.formatVer, file.data, piece.expiration, publicKey)

				}

				trashDurToUse := piece.trashDur
				dir.ReplaceTrashnow(func() time.Time {
					return time.Now().Add(-trashDurToUse)
				})
				// Trash the piece
				require.NoError(t, store.Trash(ctx, satellite.satelliteID, piece.pieceID))

				// Confirm is missing
				r, err := store.Reader(ctx, satellite.satelliteID, piece.pieceID)
				require.Error(t, err)
				require.Nil(t, r)

				// Verify no expiry information is returned for this piece
				if !piece.expiration.IsZero() {
					infos, err := store.GetExpired(ctx, time.Now().Add(720*time.Hour), 1000)
					require.NoError(t, err)
					var found bool
					for _, info := range infos {
						if info.SatelliteID == satellite.satelliteID && info.PieceID == piece.pieceID {
							found = true
						}
					}
					require.False(t, found)
				}
			}
		}

		// Initialize a trust pool
		poolConfig := trust.Config{
			CachePath: ctx.File("trust-cache.json"),
		}
		for _, satelliteURL := range satelliteURLs {
			poolConfig.Sources = append(poolConfig.Sources, &trust.StaticURLSource{URL: satelliteURL})
		}
		trust, err := trust.NewPool(zaptest.NewLogger(t), trust.Dialer(rpc.Dialer{}), poolConfig)
		require.NoError(t, err)
		require.NoError(t, trust.Refresh(ctx))

		// Empty trash by running the chore once
		trashDur := 4 * 24 * time.Hour
		chore := pieces.NewTrashChore(zaptest.NewLogger(t), 24*time.Hour, trashDur, trust, store)
		go func() {
			require.NoError(t, chore.Run(ctx))
		}()
		chore.TriggerWait(ctx)
		require.NoError(t, chore.Close())

		// Restore all pieces in the first satellite
		require.NoError(t, store.RestoreTrash(ctx, satellites[0].satelliteID))

		// Check that each piece for first satellite is back, that they are
		// MaxFormatVersionSupported (regardless of which version they began
		// with), and that signature matches.
		for _, piece := range satellites[0].pieces {
			if piece.trashDur < trashDur {
				// Expect the piece to be there
				lastFile := piece.files[len(piece.files)-1]
				verifyPieceData(ctx, t, store, satellites[0].satelliteID, piece.pieceID, filestore.MaxFormatVersionSupported, lastFile.data, piece.expiration, publicKey)
			} else {
				// Expect the piece to be missing, it should be removed from the trash on EmptyTrash
				r, err := store.Reader(ctx, satellites[1].satelliteID, piece.pieceID)
				require.Error(t, err)
				require.Nil(t, r)
			}
		}

		// Confirm 2nd satellite pieces are still in the trash
		for _, piece := range satellites[1].pieces {
			r, err := store.Reader(ctx, satellites[1].satelliteID, piece.pieceID)
			require.Error(t, err)
			require.Nil(t, r)
		}

		// Restore satellite[1] and make sure they're back (confirming they were not deleted on EmptyTrash)
		require.NoError(t, store.RestoreTrash(ctx, satellites[1].satelliteID))
		for _, piece := range satellites[1].pieces {
			if piece.trashDur < trashDur {
				// Expect the piece to be there
				lastFile := piece.files[len(piece.files)-1]
				verifyPieceData(ctx, t, store, satellites[1].satelliteID, piece.pieceID, filestore.MaxFormatVersionSupported, lastFile.data, piece.expiration, publicKey)
			} else {
				// Expect the piece to be missing, it should be removed from the trash on EmptyTrash
				r, err := store.Reader(ctx, satellites[1].satelliteID, piece.pieceID)
				require.Error(t, err)
				require.Nil(t, r)
			}
		}
	})
}

func verifyPieceData(ctx context.Context, t testing.TB, store *pieces.Store, satelliteID storj.NodeID, pieceID storj.PieceID, formatVer storage.FormatVersion, expected []byte, expiration time.Time, publicKey storj.PiecePublicKey) {
	r, err := store.ReaderWithStorageFormat(ctx, satelliteID, pieceID, formatVer)
	require.NoError(t, err)

	// Get piece hash, verify signature
	var pieceHash *pb.PieceHash
	if formatVer > filestore.FormatV0 {
		header, err := r.GetPieceHeader()
		require.NoError(t, err)
		pieceHash = &pb.PieceHash{
			PieceId:   pieceID,
			Hash:      header.GetHash(),
			PieceSize: r.Size(),
			Timestamp: header.GetCreationTime(),
			Signature: header.GetSignature(),
		}
	} else {
		info, err := store.GetV0PieceInfo(ctx, satelliteID, pieceID)
		require.NoError(t, err)
		pieceHash = info.UplinkPieceHash
	}
	require.NoError(t, signing.VerifyUplinkPieceHashSignature(ctx, publicKey, pieceHash))

	// Require piece data to match expected
	buf, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	assert.True(t, bytes.Equal(buf, expected))

	// Require expiration to match expected
	infos, err := store.GetExpired(ctx, time.Now().Add(720*time.Hour), 1000)
	require.NoError(t, err)
	var found bool
	for _, info := range infos {
		if info.SatelliteID == satelliteID && info.PieceID == pieceID {
			found = true
		}
	}
	if expiration.IsZero() {
		require.False(t, found)
	} else {
		require.True(t, found)
	}
}

func TestPieceVersionMigrate(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		const pieceSize = 1024

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		var (
			data        = testrand.Bytes(pieceSize)
			satelliteID = testrand.NodeID()
			pieceID     = testrand.PieceID()
			now         = time.Now().UTC()
		)

		// Initialize pub/priv keys for signing piece hash
		publicKeyBytes, err := hex.DecodeString("01eaebcb418cd629d4c01f365f33006c9de3ce70cf04da76c39cdc993f48fe53")
		require.NoError(t, err)
		privateKeyBytes, err := hex.DecodeString("afefcccadb3d17b1f241b7c83f88c088b54c01b5a25409c13cbeca6bfa22b06901eaebcb418cd629d4c01f365f33006c9de3ce70cf04da76c39cdc993f48fe53")
		require.NoError(t, err)
		publicKey, err := storj.PiecePublicKeyFromBytes(publicKeyBytes)
		require.NoError(t, err)
		privateKey, err := storj.PiecePrivateKeyFromBytes(privateKeyBytes)
		require.NoError(t, err)

		ol := &pb.OrderLimit{
			SerialNumber:       testrand.SerialNumber(),
			SatelliteId:        satelliteID,
			StorageNodeId:      testrand.NodeID(),
			PieceId:            pieceID,
			SatelliteSignature: []byte("sig"),
			Limit:              100,
			Action:             pb.PieceAction_GET,
			PieceExpiration:    now,
			OrderExpiration:    now,
			OrderCreation:      now,
		}
		olPieceInfo := *ol

		v0PieceInfo, ok := db.V0PieceInfo().(pieces.V0PieceInfoDBForTest)
		require.True(t, ok, "V0PieceInfoDB can not satisfy V0PieceInfoDBForTest")

		blobs, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"))
		require.NoError(t, err)
		defer ctx.Check(blobs.Close)

		store := pieces.NewStore(zaptest.NewLogger(t), blobs, v0PieceInfo, nil, nil)

		// write as a v0 piece
		tStore := &pieces.StoreForTest{store}
		writer, err := tStore.WriterForFormatVersion(ctx, satelliteID, pieceID, filestore.FormatV0)
		require.NoError(t, err)
		_, err = writer.Write(data)
		require.NoError(t, err)
		assert.Equal(t, int64(len(data)), writer.Size())
		err = writer.Commit(ctx, &pb.PieceHeader{
			Hash:         writer.Hash(),
			CreationTime: now,
			OrderLimit:   olPieceInfo,
		})
		require.NoError(t, err)

		// Create PieceHash from the v0 piece written
		ph := &pb.PieceHash{
			PieceId:   pieceID,
			Hash:      writer.Hash(),
			PieceSize: writer.Size(),
			Timestamp: now,
		}

		// sign v0 pice hash
		signedPhPieceInfo, err := signing.SignUplinkPieceHash(ctx, privateKey, ph)
		require.NoError(t, err)

		// Create v0 pieces.Info and add to v0 store
		pieceInfo := pieces.Info{
			SatelliteID:     satelliteID,
			PieceID:         pieceID,
			PieceSize:       writer.Size(),
			OrderLimit:      &olPieceInfo,
			PieceCreation:   now,
			UplinkPieceHash: signedPhPieceInfo,
		}
		require.NoError(t, v0PieceInfo.Add(ctx, &pieceInfo))

		// verify piece can be opened as v0
		tryOpeningAPiece(ctx, t, store, satelliteID, pieceID, len(data), now, filestore.FormatV0)

		// run migration
		require.NoError(t, store.MigrateV0ToV1(ctx, satelliteID, pieceID))

		// open as v1 piece
		tryOpeningAPiece(ctx, t, store, satelliteID, pieceID, len(data), now, filestore.FormatV1)

		// manually read v1 piece
		reader, err := store.ReaderWithStorageFormat(ctx, satelliteID, pieceID, filestore.FormatV1)
		require.NoError(t, err)

		// generate v1 pieceHash and verify signature is still valid
		v1Header, err := reader.GetPieceHeader()
		require.NoError(t, err)
		v1PieceHash := pb.PieceHash{
			PieceId:   v1Header.OrderLimit.PieceId,
			Hash:      v1Header.GetHash(),
			PieceSize: reader.Size(),
			Timestamp: v1Header.GetCreationTime(),
			Signature: v1Header.GetSignature(),
		}
		require.NoError(t, signing.VerifyUplinkPieceHashSignature(ctx, publicKey, &v1PieceHash))
		require.Equal(t, signedPhPieceInfo.GetSignature(), v1PieceHash.GetSignature())
		require.Equal(t, *ol, v1Header.OrderLimit)

		// Verify that it was deleted from v0PieceInfo
		retrivedInfo, err := v0PieceInfo.Get(ctx, satelliteID, pieceID)
		require.Error(t, err)
		require.Nil(t, retrivedInfo)
	})
}

// Test that the piece store can still read V0 pieces that might be left over from a previous
// version, as well as V1 pieces.
func TestMultipleStorageFormatVersions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	blobs, err := filestore.NewAt(zaptest.NewLogger(t), ctx.Dir("store"))
	require.NoError(t, err)
	defer ctx.Check(blobs.Close)

	store := pieces.NewStore(zaptest.NewLogger(t), blobs, nil, nil, nil)

	const pieceSize = 1024

	var (
		data      = testrand.Bytes(pieceSize)
		satellite = testrand.NodeID()
		v0PieceID = testrand.PieceID()
		v1PieceID = testrand.PieceID()
		now       = time.Now().UTC()
	)

	// write a V0 piece
	writeAPiece(ctx, t, store, satellite, v0PieceID, data, now, nil, filestore.FormatV0)

	// write a V1 piece
	writeAPiece(ctx, t, store, satellite, v1PieceID, data, now, nil, filestore.FormatV1)

	// look up the different pieces with Reader and ReaderWithStorageFormat
	tryOpeningAPiece(ctx, t, store, satellite, v0PieceID, len(data), now, filestore.FormatV0)
	tryOpeningAPiece(ctx, t, store, satellite, v1PieceID, len(data), now, filestore.FormatV1)

	// write a V1 piece with the same ID as the V0 piece (to simulate it being rewritten as
	// V1 during a migration)
	differentData := append(data, 111, 104, 97, 105)
	writeAPiece(ctx, t, store, satellite, v0PieceID, differentData, now, nil, filestore.FormatV1)

	// if we try to access the piece at that key, we should see only the V1 piece
	tryOpeningAPiece(ctx, t, store, satellite, v0PieceID, len(differentData), now, filestore.FormatV1)

	// unless we ask specifically for a V0 piece
	reader, err := store.ReaderWithStorageFormat(ctx, satellite, v0PieceID, filestore.FormatV0)
	require.NoError(t, err)
	verifyPieceHandle(t, reader, len(data), now, filestore.FormatV0)
	require.NoError(t, reader.Close())

	// delete the v0PieceID; both the V0 and the V1 pieces should go away
	err = store.Delete(ctx, satellite, v0PieceID)
	require.NoError(t, err)

	reader, err = store.Reader(ctx, satellite, v0PieceID)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	assert.Nil(t, reader)
}

func TestGetExpired(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		v0PieceInfo, ok := db.V0PieceInfo().(pieces.V0PieceInfoDBForTest)
		require.True(t, ok, "V0PieceInfoDB can not satisfy V0PieceInfoDBForTest")
		expirationInfo := db.PieceExpirationDB()

		store := pieces.NewStore(zaptest.NewLogger(t), db.Pieces(), v0PieceInfo, expirationInfo, db.PieceSpaceUsedDB())

		now := time.Now().UTC()
		testDates := []struct {
			years, months, days int
		}{
			{-20, -1, -2},
			{1, 6, 14},
			{0, -1, 0},
			{0, 0, 1},
		}
		testPieces := make([]pieces.Info, 4)
		for p := range testPieces {
			testPieces[p] = pieces.Info{
				SatelliteID:     testrand.NodeID(),
				PieceID:         testrand.PieceID(),
				OrderLimit:      &pb.OrderLimit{},
				UplinkPieceHash: &pb.PieceHash{},
				PieceExpiration: now.AddDate(testDates[p].years, testDates[p].months, testDates[p].days),
			}
		}

		// put testPieces 0 and 1 in the v0 pieceinfo db
		err := v0PieceInfo.Add(ctx, &testPieces[0])
		require.NoError(t, err)
		err = v0PieceInfo.Add(ctx, &testPieces[1])
		require.NoError(t, err)

		// put testPieces 2 and 3 in the piece_expirations db
		err = expirationInfo.SetExpiration(ctx, testPieces[2].SatelliteID, testPieces[2].PieceID, testPieces[2].PieceExpiration)
		require.NoError(t, err)
		err = expirationInfo.SetExpiration(ctx, testPieces[3].SatelliteID, testPieces[3].PieceID, testPieces[3].PieceExpiration)
		require.NoError(t, err)

		// GetExpired with limit 0 gives empty result
		expired, err := store.GetExpired(ctx, now, 0)
		require.NoError(t, err)
		assert.Empty(t, expired)

		// GetExpired with limit 1 gives only 1 result, although there are 2 possible
		expired, err = store.GetExpired(ctx, now, 1)
		require.NoError(t, err)
		require.Len(t, expired, 1)
		assert.Equal(t, testPieces[2].PieceID, expired[0].PieceID)
		assert.Equal(t, testPieces[2].SatelliteID, expired[0].SatelliteID)
		assert.False(t, expired[0].InPieceInfo)

		// GetExpired with 2 or more gives all expired results correctly; one from
		// piece_expirations, and one from pieceinfo
		expired, err = store.GetExpired(ctx, now, 1000)
		require.NoError(t, err)
		require.Len(t, expired, 2)
		assert.Equal(t, testPieces[2].PieceID, expired[0].PieceID)
		assert.Equal(t, testPieces[2].SatelliteID, expired[0].SatelliteID)
		assert.False(t, expired[0].InPieceInfo)
		assert.Equal(t, testPieces[0].PieceID, expired[1].PieceID)
		assert.Equal(t, testPieces[0].SatelliteID, expired[1].SatelliteID)
		assert.True(t, expired[1].InPieceInfo)
	})
}

func TestOverwriteV0WithV1(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		v0PieceInfo, ok := db.V0PieceInfo().(pieces.V0PieceInfoDBForTest)
		require.True(t, ok, "V0PieceInfoDB can not satisfy V0PieceInfoDBForTest")
		expirationInfo := db.PieceExpirationDB()

		store := pieces.NewStore(zaptest.NewLogger(t), db.Pieces(), v0PieceInfo, expirationInfo, db.PieceSpaceUsedDB())

		satelliteID := testrand.NodeID()
		pieceID := testrand.PieceID()
		v0Data := testrand.Bytes(4 * memory.MiB)
		v1Data := testrand.Bytes(3 * memory.MiB)

		// write the piece as V0. We can't provide the expireTime via writeAPiece, because
		// BlobWriter.Commit only knows how to store expiration times in piece_expirations.
		v0CreateTime := time.Now().UTC()
		v0ExpireTime := v0CreateTime.AddDate(5, 0, 0)
		writeAPiece(ctx, t, store, satelliteID, pieceID, v0Data, v0CreateTime, nil, filestore.FormatV0)
		// now put the piece in the pieceinfo db directly, because store won't do that for us.
		// this is where the expireTime takes effect.
		err := v0PieceInfo.Add(ctx, &pieces.Info{
			SatelliteID:     satelliteID,
			PieceID:         pieceID,
			PieceSize:       int64(len(v0Data)),
			PieceCreation:   v0CreateTime,
			PieceExpiration: v0ExpireTime,
			OrderLimit:      &pb.OrderLimit{},
			UplinkPieceHash: &pb.PieceHash{},
		})
		require.NoError(t, err)

		// ensure we can see it via store.Reader
		{
			reader, err := store.Reader(ctx, satelliteID, pieceID)
			require.NoError(t, err)
			assert.Equal(t, int64(len(v0Data)), reader.Size())
			assert.Equal(t, filestore.FormatV0, reader.StorageFormatVersion())
			gotData, err := ioutil.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, v0Data, gotData)
			require.NoError(t, reader.Close())
		}

		// ensure we can see it via WalkSatellitePieces
		calledTimes := 0
		err = store.WalkSatellitePieces(ctx, satelliteID, func(access pieces.StoredPieceAccess) error {
			calledTimes++
			require.Equal(t, 1, calledTimes)
			gotCreateTime, err := access.CreationTime(ctx)
			require.NoError(t, err)
			assert.Equal(t, v0CreateTime, gotCreateTime)
			_, gotSize, err := access.Size(ctx)
			require.NoError(t, err)
			assert.Equal(t, int64(len(v0Data)), gotSize)
			return nil
		})
		require.NoError(t, err)

		// now "overwrite" the piece (write a new blob with the same id, but with V1 storage)
		v1CreateTime := time.Now().UTC()
		v1ExpireTime := v1CreateTime.AddDate(5, 0, 0)
		writeAPiece(ctx, t, store, satelliteID, pieceID, v1Data, v1CreateTime, &v1ExpireTime, filestore.FormatV1)

		// ensure we can see it (the new piece) via store.Reader
		{
			reader, err := store.Reader(ctx, satelliteID, pieceID)
			require.NoError(t, err)
			assert.Equal(t, int64(len(v1Data)), reader.Size())
			assert.Equal(t, filestore.FormatV1, reader.StorageFormatVersion())
			gotData, err := ioutil.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, v1Data, gotData)
			require.NoError(t, reader.Close())
		}

		// now _both_ pieces should show up under WalkSatellitePieces. this may
		// be counter-intuitive, but the V0 piece still exists for now (so we can avoid
		// hitting the pieceinfo db with every new piece write). I believe this is OK, because
		// (a) I don't think that writing different pieces with the same piece ID is a normal
		// use case, unless we make a V0->V1 migrator tool, which should know about these
		// semantics; (b) the V0 piece should not ever become visible again to the user; it
		// should not be possible under normal conditions to delete one without deleting the
		// other.
		calledTimes = 0
		err = store.WalkSatellitePieces(ctx, satelliteID, func(access pieces.StoredPieceAccess) error {
			calledTimes++
			switch calledTimes {
			case 1:
				// expect the V1 piece
				assert.Equal(t, pieceID, access.PieceID())
				assert.Equal(t, filestore.FormatV1, access.StorageFormatVersion())
				gotCreateTime, err := access.CreationTime(ctx)
				require.NoError(t, err)
				assert.Equal(t, v1CreateTime, gotCreateTime)
				_, gotSize, err := access.Size(ctx)
				require.NoError(t, err)
				assert.Equal(t, int64(len(v1Data)), gotSize)
			case 2:
				// expect the V0 piece
				assert.Equal(t, pieceID, access.PieceID())
				assert.Equal(t, filestore.FormatV0, access.StorageFormatVersion())
				gotCreateTime, err := access.CreationTime(ctx)
				require.NoError(t, err)
				assert.Equal(t, v0CreateTime, gotCreateTime)
				_, gotSize, err := access.Size(ctx)
				require.NoError(t, err)
				assert.Equal(t, int64(len(v0Data)), gotSize)
			default:
				t.Fatalf("calledTimes should be 1 or 2, but it is %d", calledTimes)
			}
			return nil
		})
		require.NoError(t, err)

		// delete the pieceID; this should get both V0 and V1
		err = store.Delete(ctx, satelliteID, pieceID)
		require.NoError(t, err)

		err = store.WalkSatellitePieces(ctx, satelliteID, func(access pieces.StoredPieceAccess) error {
			t.Fatalf("this should not have been called. pieceID=%x, format=%d", access.PieceID(), access.StorageFormatVersion())
			return nil
		})
		require.NoError(t, err)
	})
}
