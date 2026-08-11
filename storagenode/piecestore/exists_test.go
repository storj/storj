// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"crypto/tls"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/shared/bloomfilter"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/satstore"
)

// TestBackendExistsTrash pins down the trash semantics of Exists: a trashed
// piece is reported as missing by both backends. The satellite treats missing
// as authoritative, so the two backends disagreeing here would make the same
// piece appear present or absent depending only on where it happens to live.
func TestBackendExistsTrash(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satellite := testrand.NodeID()
	pieceID := testrand.PieceID()

	t.Run("hashstore", func(t *testing.T) {
		bfm, err := retain.NewBloomFilterManager(t.TempDir(), 0)
		require.NoError(t, err)
		rtm := retain.NewRestoreTimeManager(t.TempDir())

		backend, err := NewHashStoreBackend(ctx, hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false), t.TempDir(), "", bfm, rtm, nil, nil)
		require.NoError(t, err)
		defer ctx.Check(backend.Close)

		writePiece(ctx, t, backend, satellite, pieceID)

		method, err := backend.Exists(ctx, satellite, pieceID)
		require.NoError(t, err)
		require.Equal(t, pb.StorageMethod_STORAGE_METHOD_HASHSTORE, method)

		method, err = backend.Exists(ctx, satellite, testrand.PieceID())
		require.NoError(t, err)
		require.Equal(t, pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED, method)

		// move the piece to the trash by compacting with a restore time in the
		// past and an empty bloom filter dated in the future.
		require.NoError(t, rtm.TestingSetRestoreTime(ctx, satellite, time.Now().AddDate(-1, 0, 0)))
		require.NoError(t, bfm.Queue(ctx, satellite, &pb.RetainRequest{
			CreationDate: time.Now().AddDate(1, 0, 0),
			Filter:       bloomfilter.NewOptimal(1000, 0.01).Bytes(),
		}))
		require.NoError(t, backend.dbs[satellite].Compact(ctx))

		method, err = backend.Exists(ctx, satellite, pieceID)
		require.NoError(t, err)
		require.Equal(t, pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED, method)

		// the piece is still readable, Exists just does not revive it.
		reader, err := backend.Reader(ctx, satellite, pieceID)
		require.NoError(t, err)
		require.True(t, reader.Trash())
		require.NoError(t, reader.Close())
	})

	t.Run("piecestore", func(t *testing.T) {
		backend, store := newOldBackend(ctx, t)

		writePiece(ctx, t, backend, satellite, pieceID)

		method, err := backend.Exists(ctx, satellite, pieceID)
		require.NoError(t, err)
		require.Equal(t, pb.StorageMethod_STORAGE_METHOD_PIECESTORE, method)

		method, err = backend.Exists(ctx, satellite, testrand.PieceID())
		require.NoError(t, err)
		require.Equal(t, pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED, method)

		require.NoError(t, store.Trash(ctx, satellite, pieceID, time.Now()))

		method, err = backend.Exists(ctx, satellite, pieceID)
		require.NoError(t, err)
		require.Equal(t, pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED, method)
	})
}

// TestMigratingBackendExistsOrder pins down which backend MigratingBackend.Exists
// consults first. The order is not cosmetic: it is what keeps a piece that is
// being passively migrated (old -> new) from being reported as missing, and it
// keeps a fully migrated satellite from paying for a scan of the old blob tree.
func TestMigratingBackendExistsOrder(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satellite := testrand.NodeID()

	oldBackend, _ := newOldBackend(ctx, t)

	bfm, err := retain.NewBloomFilterManager(t.TempDir(), 0)
	require.NoError(t, err)
	newBackend, err := NewHashStoreBackend(ctx, hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false), t.TempDir(), "", bfm, retain.NewRestoreTimeManager(t.TempDir()), nil, nil)
	require.NoError(t, err)
	defer ctx.Check(newBackend.Close)

	backend := NewMigratingBackend(zaptest.NewLogger(t), oldBackend, newBackend,
		satstore.NewSatelliteStore(t.TempDir(), "migrate"), nil, nil, true)

	onlyOld := testrand.PieceID()
	writePiece(ctx, t, oldBackend, satellite, onlyOld)

	onlyNew := testrand.PieceID()
	writePiece(ctx, t, newBackend, satellite, onlyNew)

	// a piece migration is in flight for: present in both stores at once.
	inBoth := testrand.PieceID()
	writePiece(ctx, t, oldBackend, satellite, inBoth)
	writePiece(ctx, t, newBackend, satellite, inBoth)

	missing := testrand.PieceID()

	check := func(pieceID storj.PieceID, expected pb.StorageMethod) {
		t.Helper()
		method, err := backend.Exists(ctx, satellite, pieceID)
		require.NoError(t, err)
		require.Equal(t, expected, method)
	}

	// with the default state reads go to the old store first, so a piece that
	// is in both is reported from there.
	check(onlyOld, pb.StorageMethod_STORAGE_METHOD_PIECESTORE)
	check(onlyNew, pb.StorageMethod_STORAGE_METHOD_HASHSTORE)
	check(inBoth, pb.StorageMethod_STORAGE_METHOD_PIECESTORE)
	check(missing, pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED)

	// once reads prefer the new store, a piece that lives there is answered
	// without touching the old store at all, but pieces that have not been
	// migrated yet are still found.
	backend.UpdateState(ctx, satellite, func(state *MigrationState) {
		state.ReadNewFirst = true
	})

	check(onlyOld, pb.StorageMethod_STORAGE_METHOD_PIECESTORE)
	check(onlyNew, pb.StorageMethod_STORAGE_METHOD_HASHSTORE)
	check(inBoth, pb.StorageMethod_STORAGE_METHOD_HASHSTORE)
	check(missing, pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED)
}

// TestTestingBackendExists checks that the piece state overrides used by tests
// are visible to Exists: a piece deleted behind the node's back has to look
// missing, while corrupting or mutating one only changes its contents.
func TestTestingBackendExists(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satellite := testrand.NodeID()

	old, _ := newOldBackend(ctx, t)
	backend := NewTestingBackend(old)
	backend.enabled.Store(true) // TestingEnableMethods only works from testplanet

	normal, deleted, corrupted, mutated := testrand.PieceID(), testrand.PieceID(), testrand.PieceID(), testrand.PieceID()
	for _, pieceID := range []storj.PieceID{normal, deleted, corrupted, mutated} {
		writePiece(ctx, t, backend, satellite, pieceID)
	}

	backend.TestingDeletePiece(satellite, deleted)
	backend.TestingCorruptPiece(satellite, corrupted)
	backend.TestingMutatePiece(satellite, mutated, func(contents []byte, header *pb.PieceHeader) {
		contents[0]++
	})

	for _, tt := range []struct {
		pieceID  storj.PieceID
		expected pb.StorageMethod
	}{
		{normal, pb.StorageMethod_STORAGE_METHOD_PIECESTORE},
		{corrupted, pb.StorageMethod_STORAGE_METHOD_PIECESTORE},
		{mutated, pb.StorageMethod_STORAGE_METHOD_PIECESTORE},
		{deleted, pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED},
		{testrand.PieceID(), pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED},
	} {
		method, err := backend.Exists(ctx, satellite, tt.pieceID)
		require.NoError(t, err)
		require.Equal(t, tt.expected, method)
	}
}

func TestEndpointExists(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	satIdent, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	satelliteCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
		State: tls.ConnectionState{PeerCertificates: satIdent.Chain()},
	})

	present := testrand.PieceID()
	hashstorePiece := testrand.PieceID()
	missing := testrand.PieceID()

	backend := &fakeExistsBackend{
		methods: map[storj.PieceID]pb.StorageMethod{
			present:        pb.StorageMethod_STORAGE_METHOD_PIECESTORE,
			hashstorePiece: pb.StorageMethod_STORAGE_METHOD_HASHSTORE,
		},
	}

	newEndpoint := func(backend PieceBackend, trusted bool) *Endpoint {
		return &Endpoint{
			log:          zaptest.NewLogger(t),
			config:       Config{ExistsCheckWorkers: 2},
			trustSource:  fakeTrustSource{trusted: trusted},
			pieceBackend: backend,
		}
	}

	t.Run("classifies every requested piece", func(t *testing.T) {
		endpoint := newEndpoint(backend, true)

		// the same piece twice, to make sure indexes and not piece IDs are
		// reported and that both entries are filled in.
		resp, err := endpoint.Exists(satelliteCtx, &pb.ExistsRequest{
			PieceIds: []storj.PieceID{present, missing, hashstorePiece, missing},
		})
		require.NoError(t, err)
		require.Equal(t, []uint32{1, 3}, sortedUint32(resp.Missing))
		require.Equal(t, []pb.StorageMethod{
			pb.StorageMethod_STORAGE_METHOD_PIECESTORE,
			pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED,
			pb.StorageMethod_STORAGE_METHOD_HASHSTORE,
			pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED,
		}, resp.StorageMethod)
	})

	t.Run("empty request", func(t *testing.T) {
		endpoint := newEndpoint(backend, true)

		resp, err := endpoint.Exists(satelliteCtx, &pb.ExistsRequest{})
		require.NoError(t, err)
		require.Empty(t, resp.Missing)
		require.Empty(t, resp.StorageMethod)
	})

	t.Run("a failed check fails the whole request", func(t *testing.T) {
		// a piece that errors must not be silently left out of missing: the
		// satellite would read that as "the node has it" and skip repair.
		checkErrs := map[storj.PieceID]error{}
		pieceIDs := []storj.PieceID{present, missing}
		for range 3 {
			failing := testrand.PieceID()
			checkErrs[failing] = errors.New("disk is gone")
			pieceIDs = append(pieceIDs, failing)
		}

		core, logs := observer.New(zap.WarnLevel)
		endpoint := newEndpoint(&fakeExistsBackend{
			methods: backend.methods,
			errs:    checkErrs,
		}, true)
		endpoint.log = zap.New(core)

		resp, err := endpoint.Exists(satelliteCtx, &pb.ExistsRequest{PieceIds: pieceIDs})
		require.Error(t, err)
		require.Nil(t, resp)
		require.Equal(t, rpcstatus.Internal, rpcstatus.Code(err))

		// one summarized log line, not one per failing piece: when the store
		// itself is unavailable every piece of the request fails.
		require.Len(t, logs.All(), 1)
		require.Equal(t, int64(3), logs.All()[0].ContextMap()["failed"])
	})

	t.Run("untrusted satellite is rejected", func(t *testing.T) {
		endpoint := newEndpoint(backend, false)

		resp, err := endpoint.Exists(satelliteCtx, &pb.ExistsRequest{
			PieceIds: []storj.PieceID{present},
		})
		require.Error(t, err)
		require.Nil(t, resp)
		require.Equal(t, rpcstatus.PermissionDenied, rpcstatus.Code(err))
	})

	t.Run("request without a peer is rejected", func(t *testing.T) {
		endpoint := newEndpoint(backend, true)

		resp, err := endpoint.Exists(ctx, &pb.ExistsRequest{
			PieceIds: []storj.PieceID{present},
		})
		require.Error(t, err)
		require.Nil(t, resp)
		require.Equal(t, rpcstatus.Unauthenticated, rpcstatus.Code(err))
	})
}

//
// helpers
//

func writePiece(ctx *testcontext.Context, t *testing.T, backend PieceBackend, satellite storj.NodeID, pieceID storj.PieceID) {
	t.Helper()

	writer, err := backend.Writer(ctx, satellite, pieceID, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
	require.NoError(t, err)
	_, err = writer.Write(testrand.Bytes(1024))
	require.NoError(t, err)
	require.NoError(t, writer.Commit(ctx, &pb.PieceHeader{
		OrderLimit:    pb.OrderLimit{PieceId: pieceID},
		HashAlgorithm: pb.PieceHashAlgorithm_BLAKE3,
		Hash:          writer.Hash(),
	}))
}

func newOldBackend(ctx *testcontext.Context, t *testing.T) (*OldPieceBackend, *pieces.Store) {
	t.Helper()

	dir, err := filestore.NewDir(zaptest.NewLogger(t), t.TempDir())
	require.NoError(t, err)
	blobs := filestore.New(zaptest.NewLogger(t), dir, filestore.DefaultConfig)
	t.Cleanup(func() { require.NoError(t, blobs.Close()) })

	store := pieces.NewStore(zaptest.NewLogger(t),
		pieces.NewFileWalker(zaptest.NewLogger(t), blobs, nil, nil, nil),
		nil, blobs, nil, nil, pieces.DefaultConfig)

	return NewOldPieceBackend(store, nil, nil), store
}

func sortedUint32(vals []uint32) []uint32 {
	out := append([]uint32(nil), vals...)
	slices.Sort(out)
	return out
}

type fakeExistsBackend struct {
	PieceBackend

	methods map[storj.PieceID]pb.StorageMethod
	errs    map[storj.PieceID]error
}

func (f *fakeExistsBackend) Exists(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (pb.StorageMethod, error) {
	if err, ok := f.errs[pieceID]; ok {
		return pb.StorageMethod_STORAGE_METHOD_UNSPECIFIED, err
	}
	return f.methods[pieceID], nil
}

type fakeTrustSource struct {
	trusted bool
}

func (f fakeTrustSource) GetSatellites(ctx context.Context) []storj.NodeID { return nil }

func (f fakeTrustSource) GetNodeURL(ctx context.Context, id storj.NodeID) (storj.NodeURL, error) {
	return storj.NodeURL{}, nil
}

func (f fakeTrustSource) VerifySatelliteID(ctx context.Context, id storj.NodeID) error {
	if !f.trusted {
		return errors.New("untrusted satellite")
	}
	return nil
}

func (f fakeTrustSource) GetSignee(ctx context.Context, id pb.NodeID) (signing.Signee, error) {
	return nil, nil
}
