// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
		mud.Select[*RecordOneOffPayments](ball),
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
		_, _, partial, err := parsePartialRange(period, "", "")
		require.NoError(t, err)
		require.False(t, partial)
	})

	t.Run("end date is inclusive", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange(period, "2026-08-01", "2026-08-10")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, date(1), start)
		require.Equal(t, date(11), endExclusive)
	})

	t.Run("single day range", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange(period, "2026-08-05", "2026-08-05")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, date(5), start)
		require.Equal(t, date(6), endExclusive)
	})

	t.Run("whole month is inside the period", func(t *testing.T) {
		start, endExclusive, partial, err := parsePartialRange(period, "2026-08-01", "2026-08-31")
		require.NoError(t, err)
		require.True(t, partial)
		require.Equal(t, date(1), start)
		require.Equal(t, period.EndDateExclusive(), endExclusive)
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
		{"start before period", "2026-07-31", "2026-08-10", "must be inside the --period"},
		{"end after period", "2026-08-20", "2026-09-01", "must be inside the --period"},
		{"range in another month", "2026-09-01", "2026-09-10", "must be inside the --period"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, partial, err := parsePartialRange(period, tt.start, tt.end)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errorMsg)
			require.False(t, partial)
		})
	}
}
