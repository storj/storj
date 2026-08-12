// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
)

// Self-signed tag names that let a storage node declare its own compensation
// rates. A tag is only honored when the node is itself the signer, and only
// when the declared value is lower than the operator-configured rate.
const (
	TagStoragePrice = "storage_price"
	TagEgressPrice  = "egress_price"
	TagRepairPrice  = "repair_price"
	TagAuditPrice   = "audit_price"
)

// hoursPerMonth is the divisor used to convert USD/TB/month (a storage node
// operator's typical unit) to USD/TB/hour. 720 = 24h * 30d, matching the CLI
// flag default for AtRestGBHours.
const hoursPerMonth = 720

// bytesPerTB is the ratio from TB to GB.
const gbPerTB = 1000

// EffectiveRates returns rates equal to min(config, self-signed tag) for each
// of the four price tags a node may declare. Tags not signed by the node
// itself are ignored. Malformed or negative values are logged and skipped;
// zero is a valid rate (the node explicitly waives payment for that dimension).
// When multiple records share a name, the newest (by SignedAt) wins.
func EffectiveRates(config Rates, nodeID storj.NodeID, tags nodeselection.NodeTags, log *zap.Logger) Rates {
	result := config
	applyLower := func(name string, current *Rate, convert func(decimal.Decimal) decimal.Decimal) {
		value, ok := newestSelfSignedTagValue(nodeID, tags, name)
		if !ok {
			return
		}
		parsed, err := decimal.NewFromString(string(value))
		if err != nil {
			log.Warn("ignoring malformed price tag",
				zap.Stringer("node_id", nodeID),
				zap.String("tag", name),
				zap.String("value", string(value)),
				zap.Error(err))
			return
		}
		if parsed.Sign() < 0 {
			log.Warn("ignoring negative price tag",
				zap.Stringer("node_id", nodeID),
				zap.String("tag", name),
				zap.String("value", string(value)))
			return
		}
		// Reject values whose magnitude is far outside any plausible price so
		// arithmetic below (Div/LessThan/rescale) cannot materialize a huge
		// big.Int coefficient. shopspring/decimal.NewFromString happily accepts
		// exponents up to ±2^31 without expanding the coefficient, so a node
		// publishing e.g. "1e-2000000000" would otherwise OOM or stall
		// generate-invoices. Exponent() is O(1) so this check is safe before
		// any Cmp/Div call.
		if exp := parsed.Exponent(); exp < -12 || exp > 6 {
			log.Warn("ignoring out-of-range price tag",
				zap.Stringer("node_id", nodeID),
				zap.String("tag", name),
				zap.String("value", string(value)))
			return
		}
		if convert != nil {
			parsed = convert(parsed)
		}
		if parsed.LessThan(decimal.Decimal(*current)) {
			*current = Rate(parsed)
		}
	}

	// storage_price is USD/TB/month; convert to USD/GB/hour to match AtRestGBHours.
	storageConvert := func(v decimal.Decimal) decimal.Decimal {
		return v.Div(decimal.NewFromInt(gbPerTB * hoursPerMonth))
	}
	applyLower(TagStoragePrice, &result.AtRestGBHours, storageConvert)
	applyLower(TagEgressPrice, &result.GetTB, nil)
	applyLower(TagRepairPrice, &result.GetRepairTB, nil)
	applyLower(TagAuditPrice, &result.GetAuditTB, nil)

	return result
}

// newestSelfSignedTagValue returns the value of the newest tag with the given
// name that was signed by the node itself.
func newestSelfSignedTagValue(nodeID storj.NodeID, tags nodeselection.NodeTags, name string) ([]byte, bool) {
	var (
		found bool
		value []byte
		when  int64
	)
	for _, tag := range tags {
		if tag.Name != name || tag.Signer != nodeID {
			continue
		}
		ts := tag.SignedAt.Unix()
		if !found || ts > when {
			found = true
			value = tag.Value
			when = ts
		}
	}
	return value, found
}
