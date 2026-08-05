// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
)

func TestResolveLabels(t *testing.T) {
	projectID := testrand.UUID()
	labels := []string{"0", "west"}

	// a backend's default label is its position, so an assignment written when
	// backends were addressed by index still resolves to the same backend
	resolved, err := resolveLabels(map[uuid.UUID]string{projectID: "0"}, labels)
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]int{projectID: 0}, resolved)

	resolved, err = resolveLabels(map[uuid.UUID]string{projectID: "west"}, labels)
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]int{projectID: 1}, resolved)

	// an unknown label must not fall back to the default backend: the project's
	// metadata simply is not in that database
	_, err = resolveLabels(map[uuid.UUID]string{projectID: "east"}, labels)
	require.Error(t, err)
}
