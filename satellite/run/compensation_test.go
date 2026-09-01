// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/cli"
	"storj.io/storj/shared/mud"
)

// Smoketest to check if the compensation subcommands are registered with all
// their dependencies.
func TestCompensation(t *testing.T) {
	ball := mud.NewBall()

	// these are provided by the CLI environment
	mud.Provide[*modular.StopTrigger](ball, func() *modular.StopTrigger {
		return &modular.StopTrigger{}
	})
	mud.Provide[*cli.ConfigDir](ball, func() *cli.ConfigDir {
		return &cli.ConfigDir{Dir: t.TempDir()}
	})
	mud.View[*cli.ConfigDir, cli.ConfigDir](ball, mud.Dereference)

	Module(ball)

	for _, selector := range []mud.ComponentSelector{
		mud.Select[*GenerateInvoices](ball),
		mud.Select[*RecordPeriod](ball),
		mud.Select[*RecordPaystubs](ball),
		mud.Select[*RecordOneOffPayments](ball),
		mud.Select[*RecordPayments](ball),
		mud.Select[*Finalize](ball),
		mud.Select[*GeneratePayments](ball),
		mud.Select[*WalletSummary](ball),
	} {
		result := mud.FindSelectedWithDependencies(ball, selector)
		require.True(t, len(result) > 0)
	}
}

func TestParsePartialRange(t *testing.T) {
	period := compensation.Period{Year: 2026, Month: time.August}
	date := func(day int) time.Time {
		return time.Date(2026, time.August, day, 0, 0, 0, 0, time.UTC)
	}

	t.Run("both empty means whole month", func(t *testing.T) {
		_, _, partial, err := parsePartialRange("", "")
		require.NoError(t, err)
		require.False(t, partial)
	})

	t.Run("end date is inclusive", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange("2026-08-01", "2026-08-10")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, date(1), start)
		require.Equal(t, date(11), endExclusive)
	})

	t.Run("single day range", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange("2026-08-05", "2026-08-05")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, date(5), start)
		require.Equal(t, date(6), endExclusive)
	})

	t.Run("whole month", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange("2026-08-01", "2026-08-31")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, date(1), start)
		require.Equal(t, period.EndDateExclusive(), endExclusive)
	})

	// A range is not required to be contained in the --period it is recorded
	// against, so one crossing a month boundary parses fine.
	t.Run("range crossing the month boundary", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange("2026-07-27", "2026-08-31")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, time.Date(2026, time.July, 27, 0, 0, 0, 0, time.UTC), start)
		require.Equal(t, period.EndDateExclusive(), endExclusive)
	})

	t.Run("range in another month", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange("2026-09-01", "2026-09-10")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC), start)
		require.Equal(t, time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC), endExclusive)
	})

	for _, tt := range []struct {
		name     string
		start    string
		end      string
		errorMsg string
	}{
		{"only start", "2026-08-01", "", "must be set together"},
		{"only end", "", "2026-08-10", "must be set together"},
		{"unparseable start", "2026-8-1", "2026-08-10", "invalid --start-date"},
		{"unparseable end", "2026-08-01", "not-a-date", "invalid --end-date"},
		{"end before start", "2026-08-10", "2026-08-01", "must be on or after"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, partial, err := parsePartialRange(tt.start, tt.end)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errorMsg)
			require.False(t, partial)
		})
	}
}

// The withholding tier and the disqualification cut-off are derived once, at
// the range end, so a range containing a step classifies its earlier days by
// the state at its end. These are the nodes that difference applies to.
func TestClassificationBoundariesInRange(t *testing.T) {
	withheldPercents := []int{75, 50, 0}
	day := func(month time.Month, d int) time.Time {
		return time.Date(2026, month, d, 0, 0, 0, 0, time.UTC)
	}
	at := func(t time.Time) *time.Time { return &t }

	// Created on 2026-07-05, so the first tier ends on 2026-08-05 and the
	// second one on 2026-09-05.
	createdAt := day(time.July, 5)

	t.Run("tier step inside the range", func(t *testing.T) {
		node := compensation.NodeInfo{ID: testrand.NodeID(), CreatedAt: createdAt}
		tierStepped, disqualified := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, withheldPercents, day(time.July, 27), day(time.September, 1))
		require.Equal(t, []storj.NodeID{node.ID}, tierStepped)
		require.Empty(t, disqualified)
	})

	t.Run("no tier step inside the range", func(t *testing.T) {
		node := compensation.NodeInfo{ID: testrand.NodeID(), CreatedAt: createdAt}
		tierStepped, disqualified := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, withheldPercents, day(time.August, 6), day(time.September, 1))
		require.Empty(t, tierStepped)
		require.Empty(t, disqualified)
	})

	// A step on the first day of the range is already reflected by every
	// classification of the range, so the range does not span it.
	t.Run("tier step on the first day of the range", func(t *testing.T) {
		node := compensation.NodeInfo{ID: testrand.NodeID(), CreatedAt: createdAt}
		tierStepped, _ := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, withheldPercents, day(time.August, 5), day(time.September, 1))
		require.Empty(t, tierStepped)
	})

	t.Run("disqualified inside the range", func(t *testing.T) {
		node := compensation.NodeInfo{
			ID:           testrand.NodeID(),
			CreatedAt:    day(time.January, 1),
			Disqualified: at(day(time.August, 20)),
		}
		tierStepped, disqualified := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, withheldPercents, day(time.July, 27), day(time.September, 1))
		require.Empty(t, tierStepped)
		require.Equal(t, []storj.NodeID{node.ID}, disqualified)
	})

	t.Run("disqualified before the range", func(t *testing.T) {
		node := compensation.NodeInfo{
			ID:           testrand.NodeID(),
			CreatedAt:    day(time.January, 1),
			Disqualified: at(day(time.July, 20)),
		}
		_, disqualified := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, withheldPercents, day(time.July, 27), day(time.September, 1))
		require.Empty(t, disqualified)
	})

	// GenerateStatements exempts a gracefully exited node from the zeroing, so
	// its disqualification date makes no difference to the amounts.
	t.Run("gracefully exited node is not reported", func(t *testing.T) {
		node := compensation.NodeInfo{
			ID:           testrand.NodeID(),
			CreatedAt:    day(time.January, 1),
			Disqualified: at(day(time.August, 20)),
			GracefulExit: at(day(time.August, 25)),
		}
		_, disqualified := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, withheldPercents, day(time.July, 27), day(time.September, 1))
		require.Empty(t, disqualified)
	})

	t.Run("nil withheld percents falls back to the defaults", func(t *testing.T) {
		// The default schedule repeats each percent for three months, so the
		// first percent change of a node created on 2026-05-15 is on
		// 2026-08-15 (75 -> 50), inside the range below.
		node := compensation.NodeInfo{ID: testrand.NodeID(), CreatedAt: day(time.May, 15)}
		tierStepped, _ := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, nil, day(time.July, 27), day(time.September, 1))
		require.Equal(t, []storj.NodeID{node.ID}, tierStepped)
	})

	// The default schedule repeats each percent for three months, so crossing
	// a month boundary within one of those runs withholds the same percent
	// either way and is not worth warning about.
	t.Run("month boundary within a repeated default percent is not a step", func(t *testing.T) {
		node := compensation.NodeInfo{ID: testrand.NodeID(), CreatedAt: day(time.June, 15)}
		tierStepped, _ := classificationBoundariesInRange(
			[]compensation.NodeInfo{node}, nil, day(time.July, 27), day(time.September, 1))
		require.Empty(t, tierStepped)
	})
}

func TestSampleNodeIDs(t *testing.T) {
	t.Run("under the cap", func(t *testing.T) {
		ids := make([]storj.NodeID, 3)
		for i := range ids {
			ids[i] = testrand.NodeID()
		}
		require.Len(t, sampleNodeIDs(ids), 3)
	})

	t.Run("over the cap is cut short", func(t *testing.T) {
		ids := make([]storj.NodeID, maxSampledNodeIDs+5)
		for i := range ids {
			ids[i] = testrand.NodeID()
		}
		sampled := sampleNodeIDs(ids)
		require.Len(t, sampled, maxSampledNodeIDs+1)
		require.Equal(t, "...", sampled[maxSampledNodeIDs])
	})
}
