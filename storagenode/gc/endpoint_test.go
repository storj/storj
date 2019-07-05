package gc_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/gc"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

type Context testcontext.Context

func TestRetain(t *testing.T) {
	storagenodedbtest.Run(t, func(t *testing.T, db storagenode.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		pieceinfos := db.PieceInfo()
		store := pieces.NewStore(zaptest.NewLogger(t), db.Pieces())

		const nbPieces = 1000
		const nbPiecesToKeep = 990
		const nbOldPieces = 5 // pieces from nbPiecesToKeep + nbOldPieces to nbPieces will
		// have a recent timestamp and thus should not be deleted

		filter := bloomfilter.NewOptimal(nbPiecesToKeep, 0.1)

		pieceIDs := generateTestIDs(nbPieces)

		satellite0 := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
		satellite1 := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())

		uplink := testidentity.MustPregeneratedSignedIdentity(3, storj.LatestIDVersion())
		endpoint := gc.NewEndpoint(zaptest.NewLogger(t), store, pieceinfos)

		now := time.Now()
		oldTime := now.Add(-time.Duration(48) * time.Hour)
		recentTime := now.Add(time.Duration(48) * time.Hour)

		var pieceCreation time.Time
		// add all pieces to the node pieces info DB - but only count piece ids in filter
		for index, id := range pieceIDs {
			if index < nbPiecesToKeep {
				filter.Add(id)
			}

			if index < nbPiecesToKeep+nbOldPieces {
				pieceCreation = oldTime
			} else {
				pieceCreation = recentTime
			}

			piecehash0, err := signing.SignPieceHash(ctx,
				signing.SignerFromFullIdentity(uplink),
				&pb.PieceHash{
					PieceId: id,
					Hash:    []byte{0, 2, 3, 4, 5},
				})
			require.NoError(t, err)

			piecehash1, err := signing.SignPieceHash(ctx,
				signing.SignerFromFullIdentity(uplink),
				&pb.PieceHash{
					PieceId: id,
					Hash:    []byte{0, 2, 3, 4, 5},
				})
			require.NoError(t, err)

			pieceinfo0 := pieces.Info{
				SatelliteID:     satellite0.ID,
				PieceSize:       4,
				PieceID:         id,
				PieceExpiration: &pieceCreation,
				UplinkPieceHash: piecehash0,
				Uplink:          uplink.PeerIdentity(),
			}
			pieceinfo1 := pieces.Info{
				SatelliteID:     satellite1.ID,
				PieceSize:       4,
				PieceID:         id,
				PieceExpiration: &pieceCreation,
				UplinkPieceHash: piecehash1,
				Uplink:          uplink.PeerIdentity(),
			}
			err = endpoint.PieceInfo().Add(ctx, &pieceinfo0)
			require.NoError(t, err)

			err = endpoint.PieceInfo().Add(ctx, &pieceinfo1)
			require.NoError(t, err)

		}

		ctx_satellite0 := peer.NewContext(ctx, &peer.Peer{
			AuthInfo: credentials.TLSInfo{
				State: tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{satellite0.PeerIdentity().Leaf, satellite0.PeerIdentity().CA},
				},
			},
		})

		retainReq := pb.RetainRequest{}
		retainReq.Filter = filter.Bytes()
		retainReq.CreationDate = now

		_, err := endpoint.Retain(ctx_satellite0, &retainReq)
		require.NoError(t, err)

		// check we have deleted nothing for satellite1
		satellite1Pieces, err := endpoint.PieceInfo().GetPiecesID(ctx, satellite1.ID, recentTime.Add(time.Duration(5)*time.Second))
		require.NoError(t, err)
		assert.Equal(t, len(satellite1Pieces), nbPieces)

		// check we did not delete recent pieces
		satellite0Pieces, err := endpoint.PieceInfo().GetPiecesID(ctx, satellite0.ID, recentTime.Add(time.Duration(5)*time.Second))
		require.NoError(t, err)

		for _, id := range pieceIDs[:nbPiecesToKeep] {
			assert.Contains(t, satellite0Pieces, id, "piece should not have been deleted (not in bloom filter)")
		}

		for _, id := range pieceIDs[nbPiecesToKeep+nbOldPieces-1:] {
			fmt.Println("here")
			assert.Contains(t, satellite0Pieces, id, "piece should not have been deleted (recent piece)")
		}
	})
}

// generateTestIDs generates n piece ids
func generateTestIDs(n int) []storj.PieceID {
	ids := make([]storj.PieceID, n)
	for i := range ids {
		ids[i] = testrand.PieceID()
	}
	return ids
}
