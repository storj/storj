// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
)

func TestEndpoint_ConvertMetabaseErr(t *testing.T) {
	endpoint := metainfo.TestingNewAPIKeysEndpoint(zaptest.NewLogger(t), nil)

	type test struct {
		err    error
		expect string
	}

	wrapClass := errs.Class("wrap")

	for _, tc := range []test{
		{err: metabase.ErrObjectNotFound.New("sql"), expect: "object not found: sql"},
		{err: wrapClass.Wrap(metabase.ErrObjectNotFound.New("sql")), expect: "object not found: wrap: object not found: sql"},
		{err: metabase.ErrSegmentNotFound.New("sql"), expect: "segment not found: sql"},
		{err: wrapClass.Wrap(metabase.ErrSegmentNotFound.New("sql")), expect: "segment not found: wrap: segment not found: sql"},
	} {
		out := endpoint.ConvertMetabaseErr(tc.err)
		assert.Equal(t, tc.expect, out.Error())
	}
}
