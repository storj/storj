// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/metabase/rangedloop/rangedlooptest"
)

func TestAvro(t *testing.T) {
	// this test is using Avro files exported from Spanner instance
	// test data was created manually and contains:
	// * 10 segments
	// * 10 segments with expiration date set
	// * corrsponding nodes	aliases

	ctx := testcontext.New(t)
	for _, batchSize := range []int{0, 1, 2, 3, 5, 10, 100} {
		config := rangedloop.Config{
			Parallelism: 2,
			BatchSize:   batchSize,
		}

		segmentsAvroIterator := rangedloop.NewAvroFileIterator("./testdata/*segments.avro*")
		nodeAliasesAvroIterator := rangedloop.NewAvroFileIterator("./testdata/*node_aliases.avro*")

		splitter := rangedloop.NewAvroSegmentsSplitter(segmentsAvroIterator, nodeAliasesAvroIterator)
		countObserver := &rangedlooptest.CountObserver{}
		service := rangedloop.NewService(zaptest.NewLogger(t), config, splitter, []rangedloop.Observer{
			countObserver,
		})

		_, err := service.RunOnce(ctx)
		require.NoError(t, err)

		require.Equal(t, 20, countObserver.NumSegments)
	}

}
