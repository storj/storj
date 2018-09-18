// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/paths"
)

func TestMeta(t *testing.T) {
	for i, test := range []metaTestStruct{
		{paths.New(""), 1, []byte(""), "a segment error",
			nil, errors.New("a segment error"), nil, nil},
		{paths.New(""), 0, []byte(""), "segment size must be larger than 0",
			nil, nil, nil, nil},
		{paths.New(""), 10, []byte("data"), "proto: streams.MetaStreamInfo: wiretype end group for non-group",
			nil, nil, nil, nil},
	} {
		errTag := fmt.Sprintf("Test case #%d", i)

		segment := &segmentStub{}
		segment.mc = makeMetaClosure(makeSegmentMeta(test.size, test.data), test.errorString)

		stream, err := NewStreamStore(segment, test.size)
		if err != nil {
			if err.Error() == test.errorString {
				continue
			}
			assert.Empty(t, stream, errTag)
			t.Fatal(err)
		}

		meta, err := stream.Meta(ctx, test.path)
		if err != nil {
			if err.Error() == test.errorString {
				assert.Equal(t, test.errorString, err.Error(), errTag)
				continue
			}

			assert.Empty(t, stream, errTag)
			t.Fatal(err)
		}
		fmt.Printf("meta: %v", meta)
		// println(meta)

	}

}
