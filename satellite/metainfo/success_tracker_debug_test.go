// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/trust"
)

func TestTrackerInfoExtension(t *testing.T) {
	dedicated := testrand.NodeID()
	trackers := NewTrackers(Config{}, []storj.NodeID{dedicated}, func(id storj.NodeID) SuccessTracker {
		return NewPercentSuccessTracker()
	}, NewPercentSuccessTracker(), NewPercentSuccessTracker(), trust.NewTrustedPeerList(nil))

	get := func(ext *TrackerInfoExtension) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		ext.Handler(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/trackers", nil))
		return rec
	}

	ext := &TrackerInfoExtension{}
	require.Equal(t, http.StatusNotFound, get(ext).Code)

	ext.Set(NewTrackerInfo(trackers, nil))
	rec := get(ext)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "/trackers?success="+dedicated.String())
}
