// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/nodeselection"
)

func TestEffectiveRates_NoTags(t *testing.T) {
	config := configRates()
	nodeID := testrand.NodeID()

	got := compensation.EffectiveRates(config, nodeID, nil, zap.NewNop())
	assert.Equal(t, config, got)
}

func TestEffectiveRates_LowerTagWins(t *testing.T) {
	config := configRates()
	nodeID := testrand.NodeID()

	// storage_price: 1.5 USD/TB/month → 1.5/1000/720 ≈ 2.083e-6 USD/GB/hour,
	// which is below config's 5e-6.
	tags := nodeselection.NodeTags{
		selfSigned(nodeID, compensation.TagStoragePrice, "1.5"),
		selfSigned(nodeID, compensation.TagEgressPrice, "10.00"),
		selfSigned(nodeID, compensation.TagRepairPrice, "5.00"),
		selfSigned(nodeID, compensation.TagAuditPrice, "4.00"),
	}

	got := compensation.EffectiveRates(config, nodeID, tags, zap.NewNop())

	assert.True(t, decimal.Decimal(got.AtRestGBHours).Equal(
		decimal.RequireFromString("1.5").Div(decimal.NewFromInt(1000*720))))
	assert.True(t, decimal.Decimal(got.GetTB).Equal(decimal.RequireFromString("10.00")))
	assert.True(t, decimal.Decimal(got.GetRepairTB).Equal(decimal.RequireFromString("5.00")))
	assert.True(t, decimal.Decimal(got.GetAuditTB).Equal(decimal.RequireFromString("4.00")))
	// PutTB and PutRepairTB are never affected.
	assert.Equal(t, config.PutTB, got.PutTB)
	assert.Equal(t, config.PutRepairTB, got.PutRepairTB)
}

func TestEffectiveRates_HigherTagIgnored(t *testing.T) {
	config := configRates()
	nodeID := testrand.NodeID()

	// egress_price above config's 20 USD/TB — should be ignored.
	tags := nodeselection.NodeTags{
		selfSigned(nodeID, compensation.TagEgressPrice, "999"),
	}

	got := compensation.EffectiveRates(config, nodeID, tags, zap.NewNop())
	assert.Equal(t, config, got)
}

func TestEffectiveRates_MalformedIgnoredWithWarning(t *testing.T) {
	config := configRates()
	nodeID := testrand.NodeID()
	logger := zaptest.NewLogger(t)

	tags := nodeselection.NodeTags{
		selfSigned(nodeID, compensation.TagEgressPrice, "not-a-number"),
		selfSigned(nodeID, compensation.TagAuditPrice, "-1.0"),
	}

	got := compensation.EffectiveRates(config, nodeID, tags, logger)
	assert.Equal(t, config, got)
}

func TestEffectiveRates_OutOfRangeExponentIgnored(t *testing.T) {
	config := configRates()
	nodeID := testrand.NodeID()
	logger := zaptest.NewLogger(t)

	// Node-controlled values with extreme exponents that decimal.NewFromString
	// parses without expanding the coefficient (up to ±2^31). Without the
	// bounds check these reach Div/LessThan and materialize multi-GB big.Ints
	// (a DoS vector).
	tags := nodeselection.NodeTags{
		selfSigned(nodeID, compensation.TagEgressPrice, "1e-2000000000"),
		selfSigned(nodeID, compensation.TagStoragePrice, "1e999999999"),
		selfSigned(nodeID, compensation.TagRepairPrice, "1e100"),
		selfSigned(nodeID, compensation.TagAuditPrice, "1e-100"),
	}

	got := compensation.EffectiveRates(config, nodeID, tags, logger)
	assert.Equal(t, config, got)
}

func TestEffectiveRates_NonSelfSignedIgnored(t *testing.T) {
	config := configRates()
	nodeID := testrand.NodeID()
	other := testrand.NodeID()

	tags := nodeselection.NodeTags{
		{
			NodeID:   nodeID,
			Signer:   other, // signed by someone else
			Name:     compensation.TagEgressPrice,
			Value:    []byte("1.00"),
			SignedAt: time.Now(),
		},
	}

	got := compensation.EffectiveRates(config, nodeID, tags, zap.NewNop())
	assert.Equal(t, config, got)
}

func TestEffectiveRates_NewestWins(t *testing.T) {
	config := configRates()
	nodeID := testrand.NodeID()
	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	tags := nodeselection.NodeTags{
		selfSignedAt(nodeID, compensation.TagEgressPrice, "1.00", old),
		selfSignedAt(nodeID, compensation.TagEgressPrice, "2.00", newer),
	}

	got := compensation.EffectiveRates(config, nodeID, tags, zap.NewNop())
	assert.True(t, decimal.Decimal(got.GetTB).Equal(decimal.RequireFromString("2.00")))
}

func TestGenerateStatements_VoluntaryDiscount(t *testing.T) {
	const (
		GB = 1_000_000_000
		TB = 1_000_000_000_000
	)

	rates := compensation.Rates{
		AtRestGBHours: compensation.RequireRateFromString("2"),
		GetTB:         compensation.RequireRateFromString("20"),
		PutTB:         compensation.RequireRateFromString("0"),
		GetRepairTB:   compensation.RequireRateFromString("10"),
		PutRepairTB:   compensation.RequireRateFromString("0"),
		GetAuditTB:    compensation.RequireRateFromString("10"),
	}

	nodeID := testrand.NodeID()

	statements, err := compensation.GenerateStatements(compensation.PeriodInfo{
		Period: compensation.Period{Year: 2025, Month: 6},
		Nodes: []compensation.NodeInfo{{
			ID:                 nodeID,
			LastContactSuccess: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			CreatedAt:          time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			UsageAtRest:        1 * GB,
			UsageGet:           1 * TB,
			Tags: nodeselection.NodeTags{
				selfSigned(nodeID, compensation.TagEgressPrice, "10"), // half of config → $10 saved on 1TB egress
			},
		}},
		Rates:            &rates,
		WithheldPercents: []int{0}, // no withholding for simplicity
	})
	require.NoError(t, err)
	require.Len(t, statements, 1)

	// Actual comp-get is $10 (1TB × $10/TB), config-comp-get is $20 → discount $10.
	assert.Equal(t, D(10), statements[0].Get)
	assert.Equal(t, D(10), statements[0].VoluntaryDiscount)
}

func TestGenerateStatements_NoDiscountWhenNoTags(t *testing.T) {
	const TB = 1_000_000_000_000

	rates := compensation.Rates{
		GetTB: compensation.RequireRateFromString("20"),
	}
	nodeID := testrand.NodeID()

	statements, err := compensation.GenerateStatements(compensation.PeriodInfo{
		Period: compensation.Period{Year: 2025, Month: 6},
		Nodes: []compensation.NodeInfo{{
			ID:                 nodeID,
			LastContactSuccess: time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			CreatedAt:          time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			UsageGet:           1 * TB,
		}},
		Rates:            &rates,
		WithheldPercents: []int{0},
	})
	require.NoError(t, err)
	require.Len(t, statements, 1)
	assert.Equal(t, D(0), statements[0].VoluntaryDiscount)
}

func configRates() compensation.Rates {
	return compensation.Rates{
		AtRestGBHours: compensation.RequireRateFromString("0.000005"),
		GetTB:         compensation.RequireRateFromString("20.00"),
		PutTB:         compensation.RequireRateFromString("0"),
		GetRepairTB:   compensation.RequireRateFromString("10.00"),
		PutRepairTB:   compensation.RequireRateFromString("0"),
		GetAuditTB:    compensation.RequireRateFromString("10.00"),
	}
}

func selfSigned(nodeID storj.NodeID, name, value string) nodeselection.NodeTag {
	return selfSignedAt(nodeID, name, value, time.Now())
}

func selfSignedAt(nodeID storj.NodeID, name, value string, at time.Time) nodeselection.NodeTag {
	return nodeselection.NodeTag{
		NodeID:   nodeID,
		Signer:   nodeID,
		Name:     name,
		Value:    []byte(value),
		SignedAt: at,
	}
}
