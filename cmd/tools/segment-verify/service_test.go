// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	segmentverify "storj.io/storj/cmd/tools/segment-verify"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

func TestService_EmptyRange(t *testing.T) {
	ctx := testcontext.New(t)
	log := testplanet.NewLogger(t)

	config := segmentverify.ServiceConfig{
		NotFoundPath:      ctx.File("not-found.csv"),
		RetryPath:         ctx.File("retry.csv"),
		ProblemPiecesPath: ctx.File("problem-pieces.csv"),
		MaxOffline:        2,
	}

	metabase := newMetabaseMock(map[metabase.NodeAlias]storj.NodeID{})
	verifier := &verifierMock{allSuccess: true}

	service, err := segmentverify.NewService(log.Named("segment-verify"), metabase, verifier, metabase, config)
	require.NoError(t, err)
	require.NotNil(t, service)

	defer ctx.Check(service.Close)

	err = service.ProcessRange(ctx, uuid.UUID{}, uuid.Max())
	require.NoError(t, err)
}

func TestService_Success(t *testing.T) {
	ctx := testcontext.New(t)
	log := testplanet.NewLogger(t)

	config := segmentverify.ServiceConfig{
		NotFoundPath:      ctx.File("not-found.csv"),
		RetryPath:         ctx.File("retry.csv"),
		ProblemPiecesPath: ctx.File("problem-pieces.csv"),
		PriorityNodesPath: ctx.File("priority-nodes.txt"),

		Check:       3,
		BatchSize:   100,
		Concurrency: 3,
		MaxOffline:  2,
	}

	// the node 1 is going to be priority
	err := os.WriteFile(config.PriorityNodesPath, []byte((storj.NodeID{1}).String()+"\n"), 0755)
	require.NoError(t, err)

	func() {
		nodes := map[metabase.NodeAlias]storj.NodeID{}
		for i := 1; i <= 0xFF; i++ {
			nodes[metabase.NodeAlias(i)] = storj.NodeID{byte(i)}
		}

		segments := []metabase.VerifySegment{
			{
				StreamID:    uuid.UUID{0x10, 0x10},
				AliasPieces: metabase.AliasPieces{{Number: 1, Alias: 8}, {Number: 3, Alias: 9}, {Number: 5, Alias: 10}, {Number: 0, Alias: 1}},
			},
			{
				StreamID:    uuid.UUID{0x20, 0x20},
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 2}, {Number: 1, Alias: 3}, {Number: 7, Alias: 4}},
			},
			{ // this won't get processed due to the high limit
				StreamID:    uuid.UUID{0x30, 0x30},
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 2}, {Number: 1, Alias: 3}, {Number: 7, Alias: 4}},
			},
		}

		metabase := newMetabaseMock(nodes, segments...)
		verifier := &verifierMock{allSuccess: true}

		service, err := segmentverify.NewService(log.Named("segment-verify"), metabase, verifier, metabase, config)
		require.NoError(t, err)
		require.NotNil(t, service)

		defer ctx.Check(service.Close)

		err = service.ProcessRange(ctx, uuid.UUID{0x10, 0x10}, uuid.UUID{0x30, 0x30})
		require.NoError(t, err)

		for node, list := range verifier.processed {
			assert.True(t, isUnique(list), "each node should process only once: %v %#v", node, list)
		}

		// node 1 is a priority node in the segments[0]
		assert.Len(t, verifier.processed[nodes[1]], 1)
		// we should get two other checks against the nodes in segments[8-10]
		assert.Equal(t, 2,
			len(verifier.processed[nodes[8]])+len(verifier.processed[nodes[9]])+len(verifier.processed[nodes[10]]),
		)
		// these correspond to checks against segment[1]
		assert.Len(t, verifier.processed[nodes[2]], 1)
		assert.Len(t, verifier.processed[nodes[3]], 1)
		assert.Len(t, verifier.processed[nodes[4]], 1)
	}()

	retryCSV, err := os.ReadFile(config.RetryPath)
	require.NoError(t, err)
	require.Equal(t, "stream id,position,created_at,required,found,not found,retry\n", string(retryCSV))

	notFoundCSV, err := os.ReadFile(config.NotFoundPath)
	require.NoError(t, err)
	require.Equal(t, "stream id,position,created_at,required,found,not found,retry\n", string(notFoundCSV))
}

func TestService_Buckets_Success(t *testing.T) {
	ctx := testcontext.New(t)
	log := testplanet.NewLogger(t)

	config := segmentverify.ServiceConfig{
		NotFoundPath:      ctx.File("not-found.csv"),
		RetryPath:         ctx.File("retry.csv"),
		ProblemPiecesPath: ctx.File("problem-pieces.csv"),
		PriorityNodesPath: ctx.File("priority-nodes.txt"),

		Check:       3,
		BatchSize:   100,
		Concurrency: 3,
		MaxOffline:  2,
	}

	// the node 1 is going to be priority
	err := os.WriteFile(config.PriorityNodesPath, []byte((storj.NodeID{1}).String()+"\n"), 0755)
	require.NoError(t, err)

	projectA := uuid.UUID{1}
	projectB := uuid.UUID{2}

	content := hex.EncodeToString(projectA[:]) + ",67616c617879\n" +
		hex.EncodeToString(projectB[:]) + ",7368696e6f6269"

	bucketListPath := ctx.File("buckets.csv")
	err = os.WriteFile(bucketListPath, []byte(content), 0755)
	require.NoError(t, err)

	func() {
		nodes := map[metabase.NodeAlias]storj.NodeID{}
		for i := 1; i <= 0xFF; i++ {
			nodes[metabase.NodeAlias(i)] = storj.NodeID{byte(i)}
		}

		segments := []metabase.VerifySegment{
			{
				StreamID:    uuid.UUID{0x10, 0x10},
				AliasPieces: metabase.AliasPieces{{Number: 1, Alias: 8}, {Number: 3, Alias: 9}, {Number: 5, Alias: 10}, {Number: 0, Alias: 1}},
			},
			{
				StreamID:    uuid.UUID{0x20, 0x20},
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 2}, {Number: 1, Alias: 3}, {Number: 7, Alias: 4}},
			},
			{ // this won't get processed because it's in non listed bucket
				StreamID:    uuid.UUID{0x30, 0x30},
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 11}, {Number: 1, Alias: 12}, {Number: 7, Alias: 13}},
			},
		}

		metabase := newMetabaseMock(nodes, segments...)
		verifier := &verifierMock{allSuccess: true}

		metabase.AddStreamIDToBucket(projectA, "67616c617879", uuid.UUID{0x10, 0x10})
		metabase.AddStreamIDToBucket(projectB, "7368696e6f6269", uuid.UUID{0x20, 0x20})
		metabase.AddStreamIDToBucket(projectB, "7777777", uuid.UUID{0x30, 0x30})

		service, err := segmentverify.NewService(log.Named("segment-verify"), metabase, verifier, metabase, config)
		require.NoError(t, err)
		require.NotNil(t, service)

		defer ctx.Check(service.Close)

		bucketList, err := service.ParseBucketFile(bucketListPath)
		require.NoError(t, err)

		err = service.ProcessBuckets(ctx, bucketList.Buckets)
		require.NoError(t, err)

		for node, list := range verifier.processed {
			assert.True(t, isUnique(list), "each node should process only once: %v %#v", node, list)
		}

		// node 1 is a priority node in the segments[0]
		assert.Len(t, verifier.processed[nodes[1]], 1)
		// we should get two other checks against the nodes in segments[8-10]
		assert.Equal(t, 2,
			len(verifier.processed[nodes[8]])+len(verifier.processed[nodes[9]])+len(verifier.processed[nodes[10]]),
		)

		// we should NOT get anything from segments[2] as it is in a different bucket
		assert.Equal(t, 0,
			len(verifier.processed[nodes[11]])+len(verifier.processed[nodes[12]])+len(verifier.processed[nodes[13]]),
		)
	}()

	retryCSV, err := os.ReadFile(config.RetryPath)
	require.NoError(t, err)
	require.Equal(t, "stream id,position,created_at,required,found,not found,retry\n", string(retryCSV))

	notFoundCSV, err := os.ReadFile(config.NotFoundPath)
	require.NoError(t, err)
	require.Equal(t, "stream id,position,created_at,required,found,not found,retry\n", string(notFoundCSV))
}

func TestService_Failures(t *testing.T) {
	ctx := testcontext.New(t)
	log := testplanet.NewLogger(t)

	config := segmentverify.ServiceConfig{
		NotFoundPath:      ctx.File("not-found.csv"),
		RetryPath:         ctx.File("retry.csv"),
		ProblemPiecesPath: ctx.File("problem-pieces.csv"),
		PriorityNodesPath: ctx.File("priority-nodes.txt"),

		Check:       2,
		BatchSize:   100,
		Concurrency: 3,
		MaxOffline:  2,
	}

	// the node 1 is going to be priority
	err := os.WriteFile(config.PriorityNodesPath, []byte((storj.NodeID{1}).String()+"\n"), 0755)
	require.NoError(t, err)

	func() {
		nodes := map[metabase.NodeAlias]storj.NodeID{}
		for i := 1; i <= 0xFF; i++ {
			nodes[metabase.NodeAlias(i)] = storj.NodeID{byte(i)}
		}

		segments := []metabase.VerifySegment{
			{
				StreamID: uuid.UUID{0x10, 0x10},
				Redundancy: storj.RedundancyScheme{
					RequiredShares: 3,
				},
				AliasPieces: metabase.AliasPieces{{Number: 1, Alias: 8}, {Number: 3, Alias: 9}, {Number: 5, Alias: 10}, {Number: 0, Alias: 1}},
			},
			{
				StreamID: uuid.UUID{0x20, 0x20},
				Redundancy: storj.RedundancyScheme{
					RequiredShares: 2,
				},
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 2}, {Number: 1, Alias: 3}, {Number: 7, Alias: 4}},
			},
			{
				StreamID: uuid.UUID{0x30, 0x30},
				Redundancy: storj.RedundancyScheme{
					RequiredShares: 2,
				},
				AliasPieces: metabase.AliasPieces{{Number: 0, Alias: 2}, {Number: 1, Alias: 3}, {Number: 7, Alias: 4}},
			},
		}

		metabase := newMetabaseMock(nodes, segments...)
		verifier := &verifierMock{
			offline:  []storj.NodeID{{0x02}, {0x08}, {0x09}, {0x0A}},
			success:  []uuid.UUID{segments[0].StreamID, segments[2].StreamID},
			notFound: []uuid.UUID{segments[1].StreamID},
		}

		service, err := segmentverify.NewService(log.Named("segment-verify"), metabase, verifier, metabase, config)
		require.NoError(t, err)
		require.NotNil(t, service)

		defer ctx.Check(service.Close)

		err = service.ProcessRange(ctx, uuid.UUID{}, uuid.Max())
		require.NoError(t, err)

		for node, list := range verifier.processed {
			assert.True(t, isUnique(list), "each node should process only once: %v %#v", node, list)
		}
	}()

	retryCSV, err := os.ReadFile(config.RetryPath)
	require.NoError(t, err)
	require.Equal(t, ""+
		"stream id,position,created_at,required,found,not found,retry\n"+
		"10100000-0000-0000-0000-000000000000,0,0001-01-01T00:00:00Z,3,1,0,1\n",
		string(retryCSV))

	notFoundCSV, err := os.ReadFile(config.NotFoundPath)
	require.NoError(t, err)
	require.Equal(t, ""+
		"stream id,position,created_at,required,found,not found,retry\n"+
		"20200000-0000-0000-0000-000000000000,0,0001-01-01T00:00:00Z,2,0,2,0\n",
		string(notFoundCSV))
}

func isUnique(segments []*segmentverify.Segment) bool {
	type segmentID struct {
		StreamID uuid.UUID
		Position metabase.SegmentPosition
	}
	seen := map[segmentID]bool{}
	for _, seg := range segments {
		id := segmentID{StreamID: seg.StreamID, Position: seg.Position}
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

type metabaseMock struct {
	nodeIDToAlias      map[storj.NodeID]metabase.NodeAlias
	aliasToNodeID      map[metabase.NodeAlias]storj.NodeID
	streamIDsPerBucket map[metabase.BucketLocation][]uuid.UUID
	segments           []metabase.VerifySegment
}

func newMetabaseMock(nodes map[metabase.NodeAlias]storj.NodeID, segments ...metabase.VerifySegment) *metabaseMock {
	mock := &metabaseMock{
		nodeIDToAlias:      map[storj.NodeID]metabase.NodeAlias{},
		aliasToNodeID:      nodes,
		segments:           segments,
		streamIDsPerBucket: make(map[metabase.BucketLocation][]uuid.UUID),
	}
	for n, id := range nodes {
		mock.nodeIDToAlias[id] = n
	}
	return mock
}

func (db *metabaseMock) AddStreamIDToBucket(projectID uuid.UUID, bucketName metabase.BucketName, streamIDs ...uuid.UUID) {
	bucket := metabase.BucketLocation{ProjectID: projectID, BucketName: bucketName}
	db.streamIDsPerBucket[bucket] = append(db.streamIDsPerBucket[bucket], streamIDs...)
}

func (db *metabaseMock) Get(ctx context.Context, nodeID storj.NodeID) (*overlay.NodeDossier, error) {
	return &overlay.NodeDossier{
		Node: pb.Node{
			Id: nodeID,
			Address: &pb.NodeAddress{
				Address: fmt.Sprintf("nodeid:%v", nodeID),
			},
		},
	}, nil
}

func (db *metabaseMock) SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf overlay.AsOfSystemTimeConfig) ([]*nodeselection.SelectedNode, error) {
	var xs []*nodeselection.SelectedNode
	for nodeID := range db.nodeIDToAlias {
		xs = append(xs, &nodeselection.SelectedNode{
			ID: nodeID,
			Address: &pb.NodeAddress{
				Address: fmt.Sprintf("nodeid:%v", nodeID),
			},
			LastNet:     "nodeid",
			LastIPPort:  fmt.Sprintf("nodeid:%v", nodeID),
			CountryCode: 0,
		})
	}
	return xs, nil
}

func (db *metabaseMock) LatestNodesAliasMap(ctx context.Context) (*metabase.NodeAliasMap, error) {
	var entries []metabase.NodeAliasEntry
	for id, alias := range db.nodeIDToAlias {
		entries = append(entries, metabase.NodeAliasEntry{
			ID:    id,
			Alias: alias,
		})
	}
	return metabase.NewNodeAliasMap(entries), nil
}

func (db *metabaseMock) DeleteSegmentByPosition(ctx context.Context, opts metabase.GetSegmentByPosition) error {
	for i, s := range db.segments {
		if opts.StreamID == s.StreamID && opts.Position == s.Position {
			db.segments = append(db.segments[:i], db.segments[i+1:]...)
			return nil
		}
	}
	return metabase.ErrSegmentNotFound.New("%v", opts)
}

func (db *metabaseMock) GetSegmentByPosition(ctx context.Context, opts metabase.GetSegmentByPosition) (segment metabase.Segment, err error) {
	s, err := db.GetSegmentByPositionForAudit(ctx, opts)
	if err != nil {
		return metabase.Segment{}, err
	}
	return metabase.Segment{
		StreamID: s.StreamID,
		Position: s.Position,
		Pieces:   s.Pieces,
	}, nil
}

func (db *metabaseMock) GetSegmentByPositionForAudit(
	ctx context.Context, opts metabase.GetSegmentByPosition,
) (segment metabase.SegmentForAudit, err error) {
	s, err := db.GetSegmentByPositionForRepair(ctx, opts)
	if err != nil {
		return metabase.SegmentForAudit{}, err
	}

	return metabase.SegmentForAudit{
		StreamID: s.StreamID,
		Position: s.Position,
		Pieces:   s.Pieces,
	}, nil

}

func (db *metabaseMock) GetSegmentByPositionForRepair(
	ctx context.Context, opts metabase.GetSegmentByPosition,
) (segment metabase.SegmentForRepair, err error) {
	for _, s := range db.segments {
		if opts.StreamID == s.StreamID && opts.Position == s.Position {
			var pieces metabase.Pieces
			for _, p := range s.AliasPieces {
				pieces = append(pieces, metabase.Piece{
					Number:      p.Number,
					StorageNode: db.aliasToNodeID[p.Alias],
				})
			}

			return metabase.SegmentForRepair{
				StreamID: s.StreamID,
				Position: s.Position,
				Pieces:   pieces,
			}, nil
		}
	}

	return metabase.SegmentForRepair{}, metabase.ErrSegmentNotFound.New("%v", opts)
}

func (db *metabaseMock) ListBucketStreamIDs(ctx context.Context, opts metabase.ListBucketStreamIDs, process func(ctx context.Context, streamIDs []uuid.UUID) error) (err error) {
	streamIDs := db.streamIDsPerBucket[opts.Bucket]
	if len(streamIDs) > 0 {
		return process(ctx, streamIDs)
	}
	return nil
}

func (db *metabaseMock) ListVerifySegments(ctx context.Context, opts metabase.ListVerifySegments) (result metabase.ListVerifySegmentsResult, err error) {
	r := metabase.ListVerifySegmentsResult{}

	for _, s := range db.segments {
		if len(opts.StreamIDs) > 0 && !slices.Contains(opts.StreamIDs, s.StreamID) {
			continue
		}

		if s.StreamID.Less(opts.CursorStreamID) {
			continue
		}
		if s.StreamID == opts.CursorStreamID && !opts.CursorPosition.Less(s.Position) {
			continue
		}

		r.Segments = append(r.Segments, s)
		if len(r.Segments) >= opts.Limit {
			break
		}
	}

	return r, nil
}

type verifierMock struct {
	allSuccess bool
	fail       error
	offline    []storj.NodeID
	success    []uuid.UUID
	notFound   []uuid.UUID

	mu        sync.Mutex
	processed map[storj.NodeID][]*segmentverify.Segment
}

func (v *verifierMock) Verify(ctx context.Context, alias metabase.NodeAlias, target storj.NodeURL, segments []*segmentverify.Segment, _ bool) (int, error) {
	v.mu.Lock()
	if v.processed == nil {
		v.processed = map[storj.NodeID][]*segmentverify.Segment{}
	}
	v.processed[target.ID] = append(v.processed[target.ID], segments...)
	v.mu.Unlock()

	for _, n := range v.offline {
		if n == target.ID {
			return 0, segmentverify.ErrNodeOffline.New("node did not respond %v", target)
		}
	}
	if v.fail != nil {
		return 0, errs.Wrap(v.fail)
	}

	if v.allSuccess {
		for _, seg := range segments {
			seg.Status.MarkFound()
		}
		return len(segments), nil
	}

	for _, seg := range v.success {
		for _, t := range segments {
			if t.StreamID == seg {
				t.Status.MarkFound()
			}
		}
	}
	for _, seg := range v.notFound {
		for _, t := range segments {
			if t.StreamID == seg {
				t.Status.MarkNotFound()
			}
		}
	}

	return len(segments), nil
}
