// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/metabasetest"
	"storj.io/storj/shared/dbutil"
)

// TestTransitionInternal_FailClosed verifies the methods that must fail closed
// on the transition adapter because a transaction cannot span two engines.
func TestTransitionInternal_FailClosed(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		ta := db.ChooseAdapter(projectID)

		// WithTx must return an error and must not invoke its body.
		called := false
		err := ta.WithTx(ctx, metabase.TransactionOptions{}, func(ctx context.Context, tx metabase.TransactionAdapter) error {
			called = true
			return nil
		})
		require.Error(t, err)
		require.False(t, called, "WithTx body must not run on the transition adapter")

		// The five internal copyObjectAdapter methods (getSegmentsForCopy,
		// getObjectNonPendingExactVersion, finalizeSegmentsCopy,
		// insertPendingCopyObject, deleteObjectExactVersion) are unexported and
		// therefore not reachable from this external (metabase_test) package.
		// Their fail-closed behavior is covered by an in-package test instead.
	})
}

// TestTransitionInternal_Lifecycle verifies the metadata / lifecycle methods of
// the transition adapter.
func TestTransitionInternal_Lifecycle(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		ta := db.ChooseAdapter(projectID)

		require.True(t, strings.HasPrefix(ta.Name(), "transition("), "Name should be prefixed with transition(")

		require.Equal(t, dbutil.Unknown, ta.Implementation())

		require.NotNil(t, ta.Config())

		now, err := ta.Now(ctx)
		require.NoError(t, err)
		require.False(t, now.IsZero())

		require.NoError(t, ta.Ping(ctx))
	})
}

// TestTransitionInternal_NodeAliases verifies node alias methods route to the
// primary backend only, leaving the secondary untouched.
func TestTransitionInternal_NodeAliases(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		ta := db.ChooseAdapter(projectID)

		nodes := []storj.NodeID{
			testrand.NodeID(),
			testrand.NodeID(),
			testrand.NodeID(),
		}

		require.NoError(t, ta.EnsureNodeAliases(ctx, metabase.EnsureNodeAliases{
			Nodes: nodes,
		}))

		// the ensured aliases are visible through the transition adapter.
		listed, err := ta.ListNodeAliases(ctx)
		require.NoError(t, err)
		require.Len(t, listed, len(nodes))

		listedIDs := map[storj.NodeID]struct{}{}
		for _, entry := range listed {
			listedIDs[entry.ID] = struct{}{}
		}
		for _, id := range nodes {
			_, ok := listedIDs[id]
			require.True(t, ok, "ensured node %v should be listed", id)
		}

		// GetNodeAliasEntries returns the requested nodes.
		entries, err := ta.GetNodeAliasEntries(ctx, metabase.GetNodeAliasEntries{
			Nodes: nodes,
		})
		require.NoError(t, err)
		require.Len(t, entries, len(nodes))

		// writes went only to the primary backend; the secondary alias table
		// must be empty.
		inPrimary, err := primary.ListNodeAliases(ctx)
		require.NoError(t, err)
		require.Len(t, inPrimary, len(nodes))

		inSecondary, err := secondary.ListNodeAliases(ctx)
		require.NoError(t, err)
		require.Empty(t, inSecondary, "node alias writes must not reach the secondary backend")
	})
}

// TestTransitionInternal_CollectBucketTallies verifies tallies are collected
// from both backends.
func TestTransitionInternal_CollectBucketTallies(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		ta := db.ChooseAdapter(projectID)

		// committed object in each backend, same project, different buckets so
		// that both produce a distinct tally.
		primaryStream := metabase.ObjectStream{
			ProjectID:  projectID,
			BucketName: "bucket-primary",
			ObjectKey:  "key-primary",
			Version:    1,
			StreamID:   testrand.UUID(),
		}
		secondaryStream := metabase.ObjectStream{
			ProjectID:  projectID,
			BucketName: "bucket-secondary",
			ObjectKey:  "key-secondary",
			Version:    1,
			StreamID:   testrand.UUID(),
		}
		require.NoError(t, primary.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
			ObjectStream: primaryStream,
			Status:       metabase.CommittedUnversioned,
			Encryption:   metabasetest.DefaultEncryption,
		}}))
		require.NoError(t, secondary.TestingBatchInsertObjects(ctx, []metabase.RawObject{{
			ObjectStream: secondaryStream,
			Status:       metabase.CommittedUnversioned,
			Encryption:   metabasetest.DefaultEncryption,
		}}))

		tallies, err := ta.CollectBucketTallies(ctx, metabase.CollectBucketTallies{
			From: metabase.BucketLocation{ProjectID: projectID},
			To:   metabase.BucketLocation{ProjectID: projectID, BucketName: "~"},
			Now:  time.Now(),
		})
		require.NoError(t, err)

		buckets := map[metabase.BucketName]struct{}{}
		for _, tally := range tallies {
			require.Equal(t, projectID, tally.ProjectID)
			buckets[tally.BucketName] = struct{}{}
		}
		require.Contains(t, buckets, metabase.BucketName("bucket-primary"))
		require.Contains(t, buckets, metabase.BucketName("bucket-secondary"))
	})
}

// TestTransitionInternal_TestingGetAllObjects verifies it returns the union of
// objects from both backends.
func TestTransitionInternal_TestingGetAllObjects(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		ta := db.ChooseAdapter(projectID)

		primaryStream := transitionSeedCommitted(ctx, t, primary, projectID, "in-primary")
		secondaryStream := transitionSeedCommitted(ctx, t, secondary, projectID, "in-secondary")

		all, err := ta.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.True(t, transitionContainsStream(all, primaryStream.StreamID), "primary object should be included")
		require.True(t, transitionContainsStream(all, secondaryStream.StreamID), "secondary object should be included")
	})
}

// TestTransitionInternal_TestingDeleteAll verifies it clears both backends.
func TestTransitionInternal_TestingDeleteAll(t *testing.T) {
	metabasetest.RunTransition(t, func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter) {
		ta := db.ChooseAdapter(projectID)

		transitionSeedCommitted(ctx, t, primary, projectID, "in-primary")
		transitionSeedCommitted(ctx, t, secondary, projectID, "in-secondary")

		require.NoError(t, ta.TestingDeleteAll(ctx))

		inPrimary, err := primary.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, inPrimary, "primary backend should be empty after TestingDeleteAll")

		inSecondary, err := secondary.TestingGetAllObjects(ctx)
		require.NoError(t, err)
		require.Empty(t, inSecondary, "secondary backend should be empty after TestingDeleteAll")
	})
}
