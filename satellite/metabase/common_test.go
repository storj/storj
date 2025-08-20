// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/uplink/private/eestream"
)

func TestParseBucketPrefixInvalid(t *testing.T) {
	var testCases = []struct {
		name   string
		prefix metabase.BucketPrefix
	}{
		{"invalid, not valid UUID", "not UUID string/bucket1"},
		{"invalid, not valid UUID, no bucket", "not UUID string"},
		{"invalid, no project, no bucket", ""},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := metabase.ParseBucketPrefix(tt.prefix)
			require.NotNil(t, err)
			require.Error(t, err)
		})
	}
}

func TestParseBucketPrefixValid(t *testing.T) {
	var testCases = []struct {
		name               string
		project            string
		bucketName         string
		expectedBucketName metabase.BucketName
	}{
		{"valid, no bucket, no objects", "bb6218e3-4b4a-4819-abbb-fa68538e33c0", "", ""},
		{"valid, with bucket", "bb6218e3-4b4a-4819-abbb-fa68538e33c0", "testbucket", "testbucket"},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			expectedProjectID, err := uuid.FromString(tt.project)
			require.NoError(t, err)
			bucketID := expectedProjectID.String() + "/" + tt.bucketName

			bucketLocation, err := metabase.ParseBucketPrefix(metabase.BucketPrefix(bucketID))
			require.NoError(t, err)
			require.Equal(t, expectedProjectID, bucketLocation.ProjectID)
			require.Equal(t, tt.expectedBucketName, bucketLocation.BucketName)
		})
	}
}

func TestParseSegmentKeyInvalid(t *testing.T) {
	var testCases = []struct {
		name       string
		segmentKey string
	}{
		{
			name:       "invalid, project ID only",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0",
		},
		{
			name:       "invalid, project ID and segment index only",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s0",
		},
		{
			name:       "invalid, project ID, bucket, and segment index only",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s0/testbucket",
		},
		{
			name:       "invalid, project ID is not UUID",
			segmentKey: "not UUID string/s0/testbucket/test/object",
		},
		{
			name:       "invalid, last segment with segment number",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/l0/testbucket/test/object",
		},
		{
			name:       "invalid, missing segment number",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s/testbucket/test/object",
		},
		{
			name:       "invalid, missing segment prefix",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/1/testbucket/test/object",
		},
		{
			name:       "invalid, segment index overflows int64",
			segmentKey: "bb6218e3-4b4a-4819-abbb-fa68538e33c0/s18446744073709551616/testbucket/test/object",
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, err := metabase.ParseSegmentKey(metabase.SegmentKey(tt.segmentKey))
			require.NotNil(t, err, tt.name)
			require.Error(t, err, tt.name)
		})
	}
}

func TestParseSegmentKeyValid(t *testing.T) {
	projectID := testrand.UUID()

	var testCases = []struct {
		name             string
		segmentKey       string
		expectedLocation metabase.SegmentLocation
	}{
		{
			name:       "valid, part 0, last segment",
			segmentKey: projectID.String() + "/l/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: metabase.LastSegmentIndex},
			},
		},
		{
			name:       "valid, part 0, last segment, trailing slash",
			segmentKey: projectID.String() + "/l/testbucket/test/object/",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object/",
				Position:   metabase.SegmentPosition{Part: 0, Index: metabase.LastSegmentIndex},
			},
		},
		{
			name:       "valid, part 0, index 0",
			segmentKey: projectID.String() + "/s0/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: 0},
			},
		},
		{
			name:       "valid, part 0, index 1",
			segmentKey: projectID.String() + "/s1/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: 1},
			},
		},
		{
			name:       "valid, part 0, index 315",
			segmentKey: projectID.String() + "/s315/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 0, Index: 315},
			},
		},
		{
			name:       "valid, part 1, index 0",
			segmentKey: projectID.String() + "/s" + strconv.FormatInt(1<<32, 10) + "/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 1, Index: 0},
			},
		},
		{
			name:       "valid, part 1, index 1",
			segmentKey: projectID.String() + "/s" + strconv.FormatInt(1<<32+1, 10) + "/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 1, Index: 1},
			},
		},
		{
			name:       "valid, part 18, index 315",
			segmentKey: projectID.String() + "/s" + strconv.FormatInt(18<<32+315, 10) + "/testbucket/test/object",
			expectedLocation: metabase.SegmentLocation{
				ProjectID:  projectID,
				BucketName: "testbucket",
				ObjectKey:  "test/object",
				Position:   metabase.SegmentPosition{Part: 18, Index: 315},
			},
		},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			segmentLocation, err := metabase.ParseSegmentKey(metabase.SegmentKey(tt.segmentKey))
			require.NoError(t, err, tt.name)
			require.Equal(t, tt.expectedLocation, segmentLocation)
		})
	}
}

func TestPiecesEqual(t *testing.T) {
	sn1 := testrand.NodeID()
	sn2 := testrand.NodeID()

	var testCases = []struct {
		source metabase.Pieces
		target metabase.Pieces
		equal  bool
	}{
		{metabase.Pieces{}, metabase.Pieces{}, true},
		{
			metabase.Pieces{
				{1, sn1},
			},
			metabase.Pieces{}, false,
		},
		{
			metabase.Pieces{},
			metabase.Pieces{
				{1, sn1},
			}, false,
		},
		{
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			},
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			}, true,
		},
		{
			metabase.Pieces{
				{2, sn2},
				{1, sn1},
			},
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			}, true,
		},
		{
			metabase.Pieces{
				{1, sn1},
				{2, sn2},
			},
			metabase.Pieces{
				{1, sn2},
				{2, sn1},
			}, false,
		},
		{
			metabase.Pieces{
				{1, sn1},
				{3, sn2},
				{2, sn2},
			},
			metabase.Pieces{
				{3, sn2},
				{1, sn1},
				{2, sn2},
			}, true,
		},
	}
	for _, tt := range testCases {
		require.Equal(t, tt.equal, tt.source.Equal(tt.target))
	}
}

func TestPiecesAdd(t *testing.T) {
	node0 := testrand.NodeID()
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()

	tests := []struct {
		name        string
		pieces      metabase.Pieces
		piecesToAdd metabase.Pieces
		want        metabase.Pieces
		wantErr     string
	}{
		{
			name: "piece exists",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			piecesToAdd: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			wantErr: "metabase: piece to add already exists (piece no: 1)",
			want:    metabase.Pieces{},
		},

		{
			name: "pieces added",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      3,
					StorageNode: node3,
				},
			},
			piecesToAdd: metabase.Pieces{
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			wantErr: "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
				metabase.Piece{
					Number:      3,
					StorageNode: node3,
				},
			},
		},
		{
			name:   "adding new pieces to empty piece",
			pieces: metabase.Pieces{},
			piecesToAdd: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
			},
			wantErr: "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
		},
		{
			name: "adding empty piece",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			piecesToAdd: metabase.Pieces{},
			wantErr:     "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
		},
		{
			name:        "adding empty piece to empty pieces",
			pieces:      metabase.Pieces{},
			piecesToAdd: metabase.Pieces{},
			wantErr:     "",
			want:        metabase.Pieces{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.pieces, tt.name)
			got, err := tt.pieces.Add(tt.piecesToAdd)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr, tt.name)
			} else {
				require.NoError(t, err, tt.name)
			}
			require.Equal(t, got, tt.want, tt.name)
		})
	}
}

func TestPiecesRemove(t *testing.T) {
	node0 := testrand.NodeID()
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()

	tests := []struct {
		name           string
		pieces         metabase.Pieces
		piecesToRemove metabase.Pieces
		want           metabase.Pieces
		wantErr        string
	}{
		{
			name:   "piece missing",
			pieces: metabase.Pieces{},
			piecesToRemove: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			wantErr: "metabase: invalid request: pieces missing",
			want:    metabase.Pieces{},
		},
		{
			name: "piecesToRemove struct is empty",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			piecesToRemove: metabase.Pieces{},
			wantErr:        "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
		},
		{
			name:           "both pieces and piecesToRemove struct are empty",
			pieces:         metabase.Pieces{},
			piecesToRemove: metabase.Pieces{},
			wantErr:        "metabase: invalid request: pieces missing",
			want:           metabase.Pieces{},
		},
		{
			name: "pieces removed",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
				metabase.Piece{
					Number:      3,
					StorageNode: node3,
				},
			},
			piecesToRemove: metabase.Pieces{
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			wantErr: "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      3,
					StorageNode: node3,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.pieces, tt.name)
			got, err := tt.pieces.Remove(tt.piecesToRemove)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr, tt.name)
			} else {
				require.NoError(t, err, tt.name)
			}
			require.Equal(t, got, tt.want, tt.name)
		})
	}
}

func TestPiecesUpdate(t *testing.T) {
	node0 := testrand.NodeID()
	node1 := testrand.NodeID()
	node2 := testrand.NodeID()
	node3 := testrand.NodeID()

	tests := []struct {
		name           string
		pieces         metabase.Pieces
		piecesToAdd    metabase.Pieces
		piecesToRemove metabase.Pieces
		want           metabase.Pieces
		wantErr        string
	}{
		{
			name: "add and remove pieces",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
			},
			piecesToRemove: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
			},
			piecesToAdd: metabase.Pieces{
				metabase.Piece{
					Number:      3,
					StorageNode: node3,
				},
			},
			wantErr: "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
				metabase.Piece{
					Number:      3,
					StorageNode: node3,
				},
			},
		},
		{
			name: "add pieces only",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
			},
			piecesToRemove: metabase.Pieces{},
			piecesToAdd: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
			},
			wantErr: "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node0,
				},
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
			},
		},
		{
			name: "remove pieces only",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
			},
			piecesToRemove: metabase.Pieces{
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
			},
			piecesToAdd: metabase.Pieces{},
			wantErr:     "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
		},
		{
			name: "both piecesToAdd and piecesToRemove are empty",
			pieces: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
			},
			piecesToRemove: metabase.Pieces{},
			piecesToAdd:    metabase.Pieces{},
			wantErr:        "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
				metabase.Piece{
					Number:      2,
					StorageNode: node2,
				},
			},
		},
		{
			name:   "updating empty pieces",
			pieces: metabase.Pieces{},
			piecesToRemove: metabase.Pieces{
				metabase.Piece{
					Number:      1,
					StorageNode: node1,
				},
			},
			piecesToAdd: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node1,
				},
			},
			wantErr: "",
			want: metabase.Pieces{
				metabase.Piece{
					Number:      0,
					StorageNode: node1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.pieces, tt.name)
			got, err := tt.pieces.Update(tt.piecesToAdd, tt.piecesToRemove)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr, tt.name)
			} else {
				require.NoError(t, err, tt.name)
			}
			require.Equal(t, got, tt.want, tt.name)
		})
	}
}

func TestStreamVersionID(t *testing.T) {
	expectedVersion := metabase.Version(1)
	expectedStreamID := uuid.UUID{2, 2, 2, 2, 2, 2, 2, 2, 4, 4, 4, 4, 4, 4, 4, 4}

	object := metabase.Object{
		ObjectStream: metabase.ObjectStream{
			Version:  expectedVersion,
			StreamID: expectedStreamID,
		},
	}
	encodedVersion := object.StreamVersionID().Bytes()
	require.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 1, 4, 4, 4, 4, 4, 4, 4, 4}, encodedVersion)

	streamVersionID, err := metabase.StreamVersionIDFromBytes(encodedVersion)
	require.NoError(t, err)
	require.Equal(t, expectedVersion, streamVersionID.Version())
	require.EqualValues(t, expectedStreamID[8:], streamVersionID.StreamIDSuffix())

	for expectedValue := range []int64{
		testrand.Int63n(math.MaxInt64),
		-1 * testrand.Int63n(math.MaxInt64), // negative version
	} {
		expectedVersion = metabase.Version(expectedValue)
		expectedStreamID = testrand.UUID()

		object = metabase.Object{
			ObjectStream: metabase.ObjectStream{
				Version:  expectedVersion,
				StreamID: expectedStreamID,
			},
		}
		encodedVersion = object.StreamVersionID().Bytes()

		streamVersionID, err = metabase.StreamVersionIDFromBytes(encodedVersion)
		require.NoError(t, err)
		require.Equal(t, expectedVersion, streamVersionID.Version())
		require.EqualValues(t, expectedStreamID[8:], streamVersionID.StreamIDSuffix())
	}
}

func TestIfNoneMatchVerify(t *testing.T) {
	var testCases = []struct {
		name     string
		input    []string
		errClass *errs.Class
	}{
		{"empty", []string{}, nil},
		{"match all", []string{"*"}, nil},
		{"match all with invalid value", []string{"*", "something"}, &metabase.ErrUnimplemented},
		{"match all with invalid values", []string{"*", "something", "else"}, &metabase.ErrUnimplemented},
		{"invalid value", []string{"something"}, &metabase.ErrUnimplemented},
		{"invalue values", []string{"something", "else"}, &metabase.ErrUnimplemented},
	}
	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			inm := metabase.IfNoneMatch(tt.input)
			if tt.errClass != nil {
				require.True(t, tt.errClass.Has(inm.Verify()))
			} else {
				require.NoError(t, inm.Verify())
			}
		})
	}
}

func BenchmarkSegmentPieceSize(b *testing.B) {
	segment := metabase.Segment{
		EncryptedSize: 64 * memory.MiB.Int32(),
		Redundancy: storj.RedundancyScheme{
			Algorithm:      storj.ReedSolomon,
			RequiredShares: 29,
			RepairShares:   35,
			OptimalShares:  80,
			TotalShares:    110,
			ShareSize:      256,
		},
	}

	b.Run("eestream.CalcPieceSize", func(b *testing.B) {
		for k := 0; k < b.N; k++ {
			redundancyScheme, _ := eestream.NewRedundancyStrategyFromStorj(segment.Redundancy)
			_ = eestream.CalcPieceSize(int64(segment.EncryptedSize), redundancyScheme)
		}
	})

	b.Run("segment.PieceSize", func(b *testing.B) {
		for k := 0; k < b.N; k++ {
			_ = segment.PieceSize()
		}
	})
}
