// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/dbutil"
)

func TestSplitLabeled(t *testing.T) {
	for _, tt := range []struct {
		name    string
		list    string
		sep     string
		want    []dbutil.Labeled
		wantErr bool
	}{
		{
			name: "unlabeled entries keep their position as label",
			list: "tidb://a:1/db; tidb://b:1/db",
			sep:  ";",
			want: []dbutil.Labeled{{"0", "tidb://a:1/db"}, {"1", "tidb://b:1/db"}},
		}, {
			// the "=" of sslmode is part of the connection string, not a label
			name: "query parameters are not labels",
			list: "postgres://host/db?sslmode=disable",
			sep:  ";",
			want: []dbutil.Labeled{{"0", "postgres://host/db?sslmode=disable"}},
		}, {
			name: "labeled entries",
			list: "west=tidb://a:1/db;east=tidb://b:1/db",
			sep:  ";",
			want: []dbutil.Labeled{{"west", "tidb://a:1/db"}, {"east", "tidb://b:1/db"}},
		}, {
			name: "labeled endpoint lists",
			list: "west=pd-a:2379,pd-b:2379; east = pd-c:2379 ",
			sep:  ";",
			want: []dbutil.Labeled{{"west", "pd-a:2379,pd-b:2379"}, {"east", "pd-c:2379"}},
		}, {
			name: "mixed forms label by position",
			list: "tidb://a:1/db;east=tidb://b:1/db",
			sep:  ";",
			want: []dbutil.Labeled{{"0", "tidb://a:1/db"}, {"east", "tidb://b:1/db"}},
		}, {
			name: "empty entries are skipped",
			list: "tidb://a:1/db;;",
			sep:  ";",
			want: []dbutil.Labeled{{"0", "tidb://a:1/db"}},
		}, {
			name:    "duplicate labels are refused",
			list:    "west=tidb://a:1/db;west=tidb://b:1/db",
			sep:     ";",
			wantErr: true,
		}, {
			// the explicit "0" collides with the position label of the first entry
			name:    "an explicit label may not collide with a positional one",
			list:    "tidb://a:1/db;0=tidb://b:1/db",
			sep:     ";",
			wantErr: true,
		}, {
			name:    "empty value",
			list:    "west=",
			sep:     ";",
			wantErr: true,
		}, {
			// project-to-adapter separates its pairs with commas, so a label
			// carrying one would name a backend nothing else can spell
			name:    "a label may not carry another list's separator",
			list:    "west,south=tidb://a:1/db",
			sep:     ";",
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dbutil.SplitLabeled(tt.list, tt.sep)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSplitLabeledGroups(t *testing.T) {
	for _, tt := range []struct {
		name     string
		list     string
		sep      string
		groupSep string
		want     []dbutil.LabeledGroup
		wantErr  bool
	}{
		{
			name:     "one entry may name several labels",
			list:     "west=pd-a:2379;east+south=pd-c:2379",
			sep:      ";",
			groupSep: "+",
			want: []dbutil.LabeledGroup{
				{Labels: []string{"west"}, Value: "pd-a:2379"},
				{Labels: []string{"east", "south"}, Value: "pd-c:2379"},
			},
		}, {
			name:     "grouped labels tolerate spaces",
			list:     " 1 + 2 = pd-a:2379,pd-b:2379 ",
			sep:      ";",
			groupSep: "+",
			want: []dbutil.LabeledGroup{
				{Labels: []string{"1", "2"}, Value: "pd-a:2379,pd-b:2379"},
			},
		}, {
			// grouping does not change what an unlabeled list means
			name:     "unlabeled entries keep their position as label",
			list:     "pd-a:2379;pd-b:2379",
			sep:      ";",
			groupSep: "+",
			want: []dbutil.LabeledGroup{
				{Labels: []string{"0"}, Value: "pd-a:2379"},
				{Labels: []string{"1"}, Value: "pd-b:2379"},
			},
		}, {
			name:     "labels stay unique across groups",
			list:     "west+east=pd-a:2379;east=pd-c:2379",
			sep:      ";",
			groupSep: "+",
			wantErr:  true,
		}, {
			name:     "an empty label in a group is refused",
			list:     "west+=pd-a:2379",
			sep:      ";",
			groupSep: "+",
			wantErr:  true,
		}, {
			// without a group separator the whole label is one label, and "+"
			// is not a character a label may carry
			name:    "grouping is off by default",
			list:    "west+east=pd-a:2379",
			sep:     ";",
			wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dbutil.SplitLabeledGroups(tt.list, tt.sep, tt.groupSep)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
