// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package auditlogger_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/admin/back-office"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/console"
)

var defaultAuditCapsConfig = auditlogger.CapsConfig{
	MaxChanges:       300,
	MaxDepth:         6,
	MaxSliceElements: 50,
	MaxMapEntries:    200,
	MaxStringLen:     512,
	MaxBytes:         28 * 1024, // 28KiB
}

func TestBuildChangeSet(t *testing.T) {
	t.Run("primitives", func(t *testing.T) {
		cases := []struct {
			name     string
			before   any
			after    any
			expected map[string]any
		}{
			{
				name:   "string change",
				before: "old",
				after:  "new",
				expected: map[string]any{
					"value": []any{"old", "new"},
				},
			},
			{
				name:   "int change",
				before: 42,
				after:  84,
				expected: map[string]any{
					"value": []any{42, 84},
				},
			},
			{
				name:     "no change",
				before:   "same",
				after:    "same",
				expected: map[string]any{},
			},
			{
				name:   "nil to value",
				before: nil,
				after:  "new",
				expected: map[string]any{
					"value": []any{nil, "new"},
				},
			},
			{
				name:     "both nil",
				before:   nil,
				after:    nil,
				expected: nil,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got := auditlogger.BuildChangeSet(tc.before, tc.after, defaultAuditCapsConfig)
				require.Equal(t, tc.expected, got)
			})
		}
	})

	t.Run("simple struct change", func(t *testing.T) {
		userID := testrand.UUID()
		before := admin.User{ID: userID, FullName: "John Doe", Email: "john@example.com"}
		after := admin.User{ID: userID, FullName: "John Smith", Email: "john@example.com"}

		got := auditlogger.BuildChangeSet(before, after, defaultAuditCapsConfig)
		expected := map[string]any{"FullName": []any{"John Doe", "John Smith"}}
		require.Equal(t, expected, got)
	})

	t.Run("user account complex struct", func(t *testing.T) {
		userID := testrand.UUID()
		projectID := testrand.UUID()
		before := admin.UserAccount{
			User: admin.User{ID: userID, FullName: "John Doe", Email: "john@example.com"},
			Kind: console.KindInfo{Name: "Free Trial", Value: console.FreeUser},
			Projects: []admin.UserProject{{
				ID: projectID, Name: "My Project",
				ProjectUsageLimits: admin.ProjectUsageLimits[int64]{StorageLimit: 1000},
			}},
			ProjectLimit: 5, MFAEnabled: false,
		}
		after := admin.UserAccount{
			User: admin.User{ID: userID, FullName: "John Smith", Email: "john@example.com"},
			Kind: console.KindInfo{Name: "Pro Account", Value: console.PaidUser},
			Projects: []admin.UserProject{{
				ID: projectID, Name: "My Project",
				ProjectUsageLimits: admin.ProjectUsageLimits[int64]{StorageLimit: 2000},
			}},
			ProjectLimit: 10, MFAEnabled: true,
		}
		got := auditlogger.BuildChangeSet(before, after, defaultAuditCapsConfig)

		require.Contains(t, got, "User.FullName")
		require.Equal(t, []any{"John Doe", "John Smith"}, got["User.FullName"])

		require.Contains(t, got, "Kind.Name")
		require.Equal(t, []any{"Free Trial", "Pro Account"}, got["Kind.Name"])
		require.Contains(t, got, "Kind.Value")

		require.Contains(t, got, "Projects[0].ProjectUsageLimits.StorageLimit")
		require.Equal(t, []any{int64(1000), int64(2000)}, got["Projects[0].ProjectUsageLimits.StorageLimit"])

		require.Contains(t, got, "ProjectLimit")
		require.Equal(t, []any{5, 10}, got["ProjectLimit"])

		require.Contains(t, got, "MFAEnabled")
		require.Equal(t, []any{false, true}, got["MFAEnabled"])
	})

	t.Run("pointers: time.Time cases", func(t *testing.T) {
		now := time.Now()
		later := now.Add(time.Hour)

		cases := []struct {
			name      string
			before    *time.Time
			after     *time.Time
			hasChange bool
		}{
			{"nil to value", nil, &now, true},
			{"value to nil", &now, nil, true},
			{"both nil", nil, nil, false},
			{"value change", &now, &later, true},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				before := admin.UserAccount{TrialExpiration: tc.before}
				after := admin.UserAccount{TrialExpiration: tc.after}
				got := auditlogger.BuildChangeSet(before, after, defaultAuditCapsConfig)
				if tc.hasChange {
					require.NotEmpty(t, got)
					require.Contains(t, got, "TrialExpiration")
				} else {
					require.Empty(t, got, "Expected no changes but got: %v", got)
				}
			})
		}
	})

	t.Run("slices: element changes & length count", func(t *testing.T) {
		p1 := testrand.UUID()
		p2 := testrand.UUID()

		cases := []struct {
			name           string
			before         []admin.UserProject
			after          []admin.UserProject
			expectCount    bool
			expectElements bool
		}{
			{
				name:        "length change",
				before:      []admin.UserProject{{ID: p1, Name: "P1"}},
				after:       []admin.UserProject{{ID: p1, Name: "P1"}, {ID: p2, Name: "P2"}},
				expectCount: true,
			},
			{
				name: "element field change",
				before: []admin.UserProject{{ID: p1, Name: "P1",
					ProjectUsageLimits: admin.ProjectUsageLimits[int64]{StorageLimit: 1000}}},
				after: []admin.UserProject{{ID: p1, Name: "P1",
					ProjectUsageLimits: admin.ProjectUsageLimits[int64]{StorageLimit: 2000}}},
				expectElements: true,
			},
			{
				name: "multiple element changes",
				before: []admin.UserProject{
					{ID: p1, Name: "P1", ProjectUsageLimits: admin.ProjectUsageLimits[int64]{StorageLimit: 1000}},
					{ID: p2, Name: "P2", ProjectUsageLimits: admin.ProjectUsageLimits[int64]{BandwidthLimit: 500}},
				},
				after: []admin.UserProject{
					{ID: p1, Name: "P1", ProjectUsageLimits: admin.ProjectUsageLimits[int64]{StorageLimit: 2000}},
					{ID: p2, Name: "P2", ProjectUsageLimits: admin.ProjectUsageLimits[int64]{BandwidthLimit: 1000}},
				},
				expectElements: true,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				before := admin.UserAccount{Projects: tc.before}
				after := admin.UserAccount{Projects: tc.after}
				got := auditlogger.BuildChangeSet(before, after, defaultAuditCapsConfig)

				if tc.expectCount {
					require.Contains(t, got, "Projects.count")
				}
				if tc.expectElements {
					hasElem := false
					for k := range got {
						if strings.HasPrefix(k, "Projects[") {
							hasElem = true
							break
						}
					}
					require.True(t, hasElem, "expected element-level changes")
				}
			})
		}
	})

	t.Run("large slice: count only", func(t *testing.T) {
		before := make([]admin.UserProject, 50)
		after := make([]admin.UserProject, 60)
		for i := 0; i < 50; i++ {
			id := testrand.UUID()
			before[i] = admin.UserProject{ID: id, Name: "Project"}
			after[i] = before[i]
		}
		for i := 50; i < 60; i++ {
			id := testrand.UUID()
			after[i] = admin.UserProject{ID: id, Name: "Project"}
		}

		got := auditlogger.BuildChangeSet(admin.UserAccount{Projects: before}, admin.UserAccount{Projects: after}, defaultAuditCapsConfig)
		require.Contains(t, got, "Projects.count")
		require.Equal(t, []any{50, 60}, got["Projects.count"])
	})

	t.Run("real-world upgrade scenario", func(t *testing.T) {
		userID := testrand.UUID()
		projectID := testrand.UUID()
		before := admin.UserAccount{
			User: admin.User{ID: userID, FullName: "Test User", Email: "test@example.com"},
			Kind: console.KindInfo{Name: "Free Trial", Value: console.FreeUser, HasPaidPrivileges: false},
			Projects: []admin.UserProject{{
				ID:                 projectID,
				Name:               "My Storj Project",
				Active:             true,
				ProjectUsageLimits: admin.ProjectUsageLimits[int64]{BandwidthLimit: 150000000000000, StorageLimit: 100000000000000, SegmentLimit: 100000001},
			}},
			ProjectLimit:    1,
			SegmentLimit:    100000001,
			TrialExpiration: nil,
			UpgradeTime:     nil,
		}
		upgradeTime := time.Now()
		after := admin.UserAccount{
			User: admin.User{ID: userID, FullName: "Test User", Email: "test@example.com"},
			Kind: console.KindInfo{Name: "Pro Account", Value: console.PaidUser, HasPaidPrivileges: true},
			Projects: []admin.UserProject{{
				ID:                 projectID,
				Name:               "My Storj Project",
				Active:             true,
				ProjectUsageLimits: admin.ProjectUsageLimits[int64]{BandwidthLimit: 150000000000000, StorageLimit: 100000000000000, SegmentLimit: 100000000},
			}},
			ProjectLimit:    3,
			SegmentLimit:    100000000,
			TrialExpiration: nil,
			UpgradeTime:     &upgradeTime,
		}
		got := auditlogger.BuildChangeSet(before, after, defaultAuditCapsConfig)

		require.Contains(t, got, "Kind.Name")
		require.Equal(t, []any{"Free Trial", "Pro Account"}, got["Kind.Name"])
		require.Contains(t, got, "Kind.Value")
		require.Equal(t, []any{console.FreeUser, console.PaidUser}, got["Kind.Value"])
		require.Contains(t, got, "Kind.HasPaidPrivileges")
		require.Equal(t, []any{false, true}, got["Kind.HasPaidPrivileges"])

		require.Contains(t, got, "ProjectLimit")
		require.Equal(t, []any{1, 3}, got["ProjectLimit"])

		require.Contains(t, got, "SegmentLimit")
		require.Equal(t, []any{int64(100000001), int64(100000000)}, got["SegmentLimit"])

		require.Contains(t, got, "UpgradeTime")

		require.Contains(t, got, "Projects[0].ProjectUsageLimits.SegmentLimit")
		require.Equal(t, []any{int64(100000001), int64(100000000)}, got["Projects[0].ProjectUsageLimits.SegmentLimit"])

		require.NotContains(t, got, "Projects")
	})

	t.Run("atomic types: time.Time root & nested", func(t *testing.T) {
		now := time.Now()
		later := now.Add(2 * time.Hour)

		// root
		res := auditlogger.BuildChangeSet(now, later, defaultAuditCapsConfig)
		require.Equal(t, map[string]any{"value": []any{now, later}}, res)

		// nested
		type S struct{ When *time.Time }
		a := S{When: &now}
		b := S{When: &later}
		res2 := auditlogger.BuildChangeSet(a, b, defaultAuditCapsConfig)
		require.Contains(t, res2, "When")
		got := res2["When"].([]any)
		require.Len(t, got, 2)
		require.Equal(t, now, got[0])
		require.Equal(t, later, got[1])
	})

	t.Run("atomic types: uuid.UUID root & nested", func(t *testing.T) {
		id1 := testrand.UUID()
		id2 := testrand.UUID()

		// root
		res := auditlogger.BuildChangeSet(id1, id2, defaultAuditCapsConfig)
		require.Equal(t, map[string]any{"value": []any{id1, id2}}, res)

		// nested (typed field, not interface{} — interface fields are skipped by design)
		type S struct{ ID uuid.UUID }
		a := S{ID: id1}
		b := S{ID: id2}
		res2 := auditlogger.BuildChangeSet(a, b, defaultAuditCapsConfig)
		require.Contains(t, res2, "ID")
		require.Equal(t, []any{id1, id2}, res2["ID"])
	})

	t.Run("opaque struct: no exported fields", func(t *testing.T) {
		type opaque struct{ x int }
		type S struct{ T opaque }
		a := S{T: opaque{x: 1}}
		b := S{T: opaque{x: 2}}
		res := auditlogger.BuildChangeSet(a, b, defaultAuditCapsConfig)
		require.Equal(t, []any{opaque{x: 1}, opaque{x: 2}}, res["T"])
	})

	t.Run("pointer policy: top-level & nested", func(t *testing.T) {
		type S struct{ N *int }
		x, y := 1, 2

		// top-level mixed kinds -> change at "value".
		res := auditlogger.BuildChangeSet(&x, y, defaultAuditCapsConfig)
		require.Equal(t, map[string]any{"value": []any{&x, y}}, res)

		// nested nil -> value.
		a := S{N: nil}
		b := S{N: &x}
		res2 := auditlogger.BuildChangeSet(a, b, defaultAuditCapsConfig)
		require.Equal(t, []any{nil, 1}, res2["N"])

		// nested both non-nil different.
		a2 := S{N: &x}
		b2 := S{N: &y}
		res3 := auditlogger.BuildChangeSet(a2, b2, defaultAuditCapsConfig)
		require.Equal(t, []any{1, 2}, res3["N"])
	})

	t.Run("maps: string-keyed vs non-string-keyed", func(t *testing.T) {
		a := map[string]int{"a": 1, "b": 2}
		b := map[string]int{"a": 1, "b": 3, "c": 9}

		res := auditlogger.BuildChangeSet(a, b, defaultAuditCapsConfig)
		require.Contains(t, res, "value.count")
		require.Equal(t, []any{2, 3}, res["value.count"])
		require.Equal(t, []any{2, 3}, res["value.b"])

		c := map[int]int{1: 10}
		d := map[int]int{1: 11}

		res2 := auditlogger.BuildChangeSet(c, d, defaultAuditCapsConfig)
		require.Equal(t, []any{c, d}, res2["value"])
	})

	t.Run("caps: MaxChanges", func(t *testing.T) {
		type S struct{ A, B, C int }
		a := S{1, 1, 1}
		b := S{2, 2, 2}

		caps := defaultAuditCapsConfig
		caps.MaxChanges = 2

		res := auditlogger.BuildChangeSet(a, b, caps)
		require.Len(t, res, 2)
		require.Contains(t, res, "A")
		require.Contains(t, res, "B")
	})

	t.Run("caps: MaxSliceElements", func(t *testing.T) {
		type E struct{ V int }
		type S struct{ L []E }
		a := S{L: []E{{1}, {1}, {1}, {1}}}
		b := S{L: []E{{2}, {2}, {2}, {2}}}

		caps := defaultAuditCapsConfig
		caps.MaxSliceElements = 2

		res := auditlogger.BuildChangeSet(a, b, caps)

		has0, has1, has2 := false, false, false
		for k := range res {
			if strings.HasPrefix(k, "L[0].") {
				has0 = true
			}
			if strings.HasPrefix(k, "L[1].") {
				has1 = true
			}
			if strings.HasPrefix(k, "L[2].") {
				has2 = true
			}
		}
		require.True(t, has0)
		require.True(t, has1)
		require.False(t, has2)
	})

	t.Run("caps: MaxMapEntries", func(t *testing.T) {
		a := map[string]int{"a": 1, "b": 1, "c": 1}
		b := map[string]int{"a": 2, "b": 2, "c": 2}

		caps := defaultAuditCapsConfig
		caps.MaxMapEntries = 2

		res := auditlogger.BuildChangeSet(a, b, caps)
		require.Contains(t, res, "value.a")
		require.Contains(t, res, "value.b")
		require.NotContains(t, res, "value.c")
	})

	t.Run("caps: MaxStringLen truncates", func(t *testing.T) {
		type S struct{ Msg string }
		longOld := strings.Repeat("x", 20)
		longNew := strings.Repeat("y", 20)
		a := S{Msg: longOld}
		b := S{Msg: longNew}

		caps := defaultAuditCapsConfig
		caps.MaxStringLen = 10 // 9 + "…"

		res := auditlogger.BuildChangeSet(a, b, caps)
		got := res["Msg"].([]any)
		require.Equal(t, "xxxxxxxxx…", got[0])
		require.Equal(t, "yyyyyyyyy…", got[1])
	})

	t.Run("caps: MaxBytes prunes", func(t *testing.T) {
		type S struct{ A, B, C, D int }
		a := S{1, 1, 1, 1}
		b := S{2, 2, 2, 2}

		caps := defaultAuditCapsConfig
		caps.MaxBytes = 60 // tiny to force pruning

		res := auditlogger.BuildChangeSet(a, b, caps)
		j, err := json.Marshal(res)
		require.NoError(t, err)
		require.LessOrEqual(t, len(j), caps.MaxBytes)
		require.NotEmpty(t, res)
	})
}
