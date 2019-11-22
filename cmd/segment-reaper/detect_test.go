// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/private/testcontext"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage/teststore"
)

func TestObserver(t *testing.T) {
	type objectSegmentRef struct {
		path    metainfo.ScopedPath
		pointer *pb.Pointer
	}

	// Creates a list of segmen references which belongs to the same object.
	// If inline is true the last segment will be of INLINE type.
	// If withNumSegments is true the last segment pointer will have 3he
	// NumberOfSegments set.
	newObject := func(numSegments int, projectID *uuid.UUID, bucketName string, inline bool, withNumSegments bool) []objectSegmentRef {
		var (
			projectIDString = projectID.String()
			references      = make([]objectSegmentRef, 0, numSegments)
		)

		var objectID string
		{
			id, err := uuid.New()
			require.NoError(t, err)
			objectID = id.String()
		}

		for i := 0; i < (numSegments - 1); i++ {
			references = append(references, objectSegmentRef{
				path: metainfo.ScopedPath{
					ProjectID:           *projectID,
					ProjectIDString:     projectIDString,
					BucketName:          bucketName,
					Segment:             fmt.Sprintf("s%d", i),
					EncryptedObjectPath: fmt.Sprintf("%s-%s-%s-s%d", projectIDString, bucketName, objectID, i),
					Raw:                 fmt.Sprintf("%s/%s/%s/s%d", projectIDString, bucketName, objectID, i),
				},
				pointer: &pb.Pointer{
					Type: pb.Pointer_REMOTE,
				},
			})
		}

		pointerType := pb.Pointer_REMOTE
		if inline {
			pointerType = pb.Pointer_INLINE
		}

		var pointerNumSegments int64
		if withNumSegments {
			pointerNumSegments = int64(numSegments)
		}

		metadata, err := proto.Marshal(&pb.StreamMeta{
			NumberOfSegments: pointerNumSegments,
		})
		require.NoError(t, err)

		return append(references, objectSegmentRef{
			path: metainfo.ScopedPath{
				ProjectID:           *projectID,
				ProjectIDString:     projectIDString,
				BucketName:          bucketName,
				Segment:             "l",
				EncryptedObjectPath: fmt.Sprintf("%s-%s-%s-l", projectIDString, bucketName, objectID),
				Raw:                 fmt.Sprintf("%s/%s/%s/l", projectIDString, bucketName, objectID),
			},
			pointer: &pb.Pointer{
				Type:     pointerType,
				Metadata: metadata,
			},
		})
	}

	t.Run("processSegment", func(t *testing.T) {
		obsvr := Observer{
			db:      teststore.New(),
			objects: make(ObjectsMap),
			// TODO: use some writer which we are able to inspect once the
			// logic of writing is implemented
			// writer: ...
		}

		t.Run("objects of different project", func(t *testing.T) {
			numSegments := rand.Intn(10) + 1
			inline := (rand.Int() % 2) == 0
			withNumSegments := (rand.Int() % 2) == 0
			projID, err := uuid.New()
			require.NoError(t, err)

			objSegmentsProj := newObject(numSegments, projID, "project1", inline, withNumSegments)
			objSegments := append([]objectSegmentRef{}, objSegmentsProj...)

			numSegments = rand.Intn(10) + 1
			inline = (rand.Int() % 2) == 0
			withNumSegments = (rand.Int() % 2) == 0
			projID, err = uuid.New()
			require.NoError(t, err)

			objSegmentsProj = newObject(numSegments, projID, "project2", inline, withNumSegments)
			objSegments = append([]objectSegmentRef{}, objSegmentsProj...)

			ctx := testcontext.New(t)
			for _, objSeg := range objSegments {
				err := obsvr.processSegment(ctx.Context, objSeg.path, objSeg.pointer)
				require.NoError(t, err)
			}

			assert.Equal(t, projID.String(), obsvr.lastProjectID, "lastProjectID")

			if inline {
				assert.Equal(t, 1, obsvr.inlineSegments, "inlineSegments")
				assert.Equal(t, 1, obsvr.lastInlineSegments, "lastInlineSegments")

				if numSegments > 1 {
					assert.Equal(t, 1, obsvr.remoteSegments, "remoteSegments")
				} else {
					assert.Zero(t, obsvr.remoteSegments, "remoteSegments")
				}
			} else {
				assert.Zero(t, obsvr.inlineSegments, "inlineSegments")
				assert.Zero(t, obsvr.lastInlineSegments, "lastInlineSegments")
				assert.Equal(t, numSegments, obsvr.remoteSegments, "remoteSegments")
			}
		})

		t.Run("object without last segment", func(t *testing.T) {
			t.Skip("TODO")
		})

		t.Run("object with non sequenced segments", func(t *testing.T) {
			t.Skip("TODO")
		})

		t.Run("object with unencrypted segments with different stored number", func(t *testing.T) {
			t.Skip("TODO")
		})
	})
}
