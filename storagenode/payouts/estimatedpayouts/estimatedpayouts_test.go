// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package estimatedpayouts_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/storagenode/payouts/estimatedpayouts"
)

func TestCurrentMonthExpectations(t *testing.T) {
	const payout = 100.0

	type test struct {
		time              time.Time
		expected          int64
		joinedAt          time.Time
		payout            estimatedpayouts.EstimatedPayout
		current, previous estimatedpayouts.PayoutMonthly
	}
	tests := []test{
		// 28 days in month
		{time.Date(2021, 2, 1, 16, 0, 0, 0, time.UTC), 4199.00, time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC),
			estimatedpayouts.EstimatedPayout{},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			}},
		{time.Date(2021, 2, 28, 10, 0, 0, 0, time.UTC), 102, time.Date(2021, 1, 26, 10, 0, 0, 0, time.UTC),
			estimatedpayouts.EstimatedPayout{},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			}},
		{time.Date(2021, 2, 28, 10, 0, 0, 0, time.UTC), 104, time.Date(2021, 2, 15, 10, 0, 0, 0, time.UTC),
			estimatedpayouts.EstimatedPayout{},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			}},
		// 31 days in month
		{time.Date(2021, 3, 1, 19, 0, 0, 0, time.UTC), 3915.0, time.Date(2021, 1, 1, 19, 0, 0, 0, time.UTC),
			estimatedpayouts.EstimatedPayout{},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			}},
		{time.Date(2021, 3, 31, 21, 0, 0, 0, time.UTC), 100, time.Date(2021, 1, 31, 21, 0, 0, 0, time.UTC),
			estimatedpayouts.EstimatedPayout{},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			}},
		{time.Date(2021, 3, 31, 21, 0, 0, 0, time.UTC), 100, time.Date(2021, 3, 15, 21, 0, 0, 0, time.UTC),
			estimatedpayouts.EstimatedPayout{},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			},
			estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
				DiskSpace:               567,
				DiskSpacePayout:         678,
				HeldRate:                789,
				Payout:                  payout,
				Held:                    901,
			}},
	}

	for _, test := range tests {
		test.payout.Set(test.current, test.previous, test.time, test.joinedAt)
		require.InDelta(t, test.expected, test.payout.CurrentMonthExpectations, 0.01)
		require.Equal(t, test.payout.CurrentMonth, test.current)
		require.Equal(t, test.payout.PreviousMonth, test.previous)
	}
}

func TestAddEstimationPayout(t *testing.T) {
	type test struct {
		basic, addition, result estimatedpayouts.EstimatedPayout
	}

	tests := []test{
		{estimatedpayouts.EstimatedPayout{
			CurrentMonth: estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   123,
				EgressRepairAudit:       123,
				EgressRepairAuditPayout: 123,
				DiskSpace:               123,
				DiskSpacePayout:         123,
				Payout:                  123,
				Held:                    123,
			},
			PreviousMonth: estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         234,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       234,
				EgressRepairAuditPayout: 234,
				DiskSpace:               234,
				DiskSpacePayout:         234,
				Payout:                  234,
				Held:                    234,
			},
			CurrentMonthExpectations: 111,
		},
			estimatedpayouts.EstimatedPayout{
				CurrentMonth: estimatedpayouts.PayoutMonthly{
					EgressBandwidth:         345,
					EgressBandwidthPayout:   345,
					EgressRepairAudit:       345,
					EgressRepairAuditPayout: 345,
					DiskSpace:               345,
					DiskSpacePayout:         345,
					Payout:                  345,
					Held:                    345,
				},
				PreviousMonth: estimatedpayouts.PayoutMonthly{
					EgressBandwidth:         456,
					EgressBandwidthPayout:   456,
					EgressRepairAudit:       456,
					EgressRepairAuditPayout: 456,
					DiskSpace:               456,
					DiskSpacePayout:         456,
					Payout:                  456,
					Held:                    456,
				},
				CurrentMonthExpectations: 222,
			},
			estimatedpayouts.EstimatedPayout{
				CurrentMonth: estimatedpayouts.PayoutMonthly{
					EgressBandwidth:         468,
					EgressBandwidthPayout:   468,
					EgressRepairAudit:       468,
					EgressRepairAuditPayout: 468,
					DiskSpace:               468,
					DiskSpacePayout:         468,
					Payout:                  468,
					Held:                    468,
				},
				PreviousMonth: estimatedpayouts.PayoutMonthly{
					EgressBandwidth:         690,
					EgressBandwidthPayout:   690,
					EgressRepairAudit:       690,
					EgressRepairAuditPayout: 690,
					DiskSpace:               690,
					DiskSpacePayout:         690,
					Payout:                  690,
					Held:                    690,
				},
				CurrentMonthExpectations: 333,
			}},
		{estimatedpayouts.EstimatedPayout{
			CurrentMonth: estimatedpayouts.PayoutMonthly{
				EgressBandwidth:         123,
				EgressBandwidthPayout:   234,
				EgressRepairAudit:       345,
				EgressRepairAuditPayout: 456,
			},
			PreviousMonth: estimatedpayouts.PayoutMonthly{
				DiskSpace:       123,
				DiskSpacePayout: 234,
				Payout:          345,
				Held:            456,
			},
			CurrentMonthExpectations: 111,
		},
			estimatedpayouts.EstimatedPayout{
				CurrentMonth: estimatedpayouts.PayoutMonthly{
					DiskSpace:       456,
					DiskSpacePayout: 345,
					Payout:          234,
					Held:            123,
				},
				PreviousMonth: estimatedpayouts.PayoutMonthly{
					EgressBandwidth:         456,
					EgressBandwidthPayout:   345,
					EgressRepairAudit:       234,
					EgressRepairAuditPayout: 123,
				},
				CurrentMonthExpectations: 111,
			},
			estimatedpayouts.EstimatedPayout{
				CurrentMonth: estimatedpayouts.PayoutMonthly{
					EgressBandwidth:         123,
					EgressBandwidthPayout:   234,
					EgressRepairAudit:       345,
					EgressRepairAuditPayout: 456,
					DiskSpace:               456,
					DiskSpacePayout:         345,
					Payout:                  234,
					Held:                    123,
				},
				PreviousMonth: estimatedpayouts.PayoutMonthly{
					EgressBandwidth:         456,
					EgressBandwidthPayout:   345,
					EgressRepairAudit:       234,
					EgressRepairAuditPayout: 123,
					DiskSpace:               123,
					DiskSpacePayout:         234,
					Payout:                  345,
					Held:                    456,
				},
				CurrentMonthExpectations: 222,
			}},
	}

	for _, test := range tests {
		test.basic.Add(test.addition)
		require.Equal(t, test.basic, test.result)
	}
}
