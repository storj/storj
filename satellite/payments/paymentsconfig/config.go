// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"

	"storj.io/common/useragent"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// Error is payments config err class.
var Error = errs.Class("payments config")

// Config defines global payments config.
type Config struct {
	Provider     string                          `help:"payments provider to use" default:""`
	MockProvider stripecoinpayments.StripeClient `internal:"true"`

	BillingConfig            billing.Config
	StripeCoinPayments       stripecoinpayments.Config
	Storjscan                storjscan.Config
	UsagePrice               ProjectUsagePrice
	BonusRate                int64                      `help:"amount of percents that user will earn as bonus credits by depositing in STORJ tokens" default:"10"`
	NodeEgressBandwidthPrice int64                      `help:"price node receive for storing TB of egress in cents" default:"2000"`
	NodeRepairBandwidthPrice int64                      `help:"price node receive for storing TB of repair in cents" default:"1000"`
	NodeAuditBandwidthPrice  int64                      `help:"price node receive for storing TB of audit in cents" default:"1000"`
	NodeDiskSpacePrice       int64                      `help:"price node receive for storing disk space in cents/TB" default:"150"`
	UsagePriceOverrides      ProjectUsagePriceOverrides `help:"semicolon-separated usage price overrides in the format partner:storage,egress,segment"`
	PackagePlans             PackagePlans               `help:"semicolon-separated partner package plans in the format partner:price,credit. Price and credit are in cents USD."`
}

// ProjectUsagePrice holds the configuration for the satellite's project usage price model.
type ProjectUsagePrice struct {
	StorageTB string `help:"price user should pay for storage per month in dollars/TB" default:"4" testDefault:"10"`
	EgressTB  string `help:"price user should pay for egress in dollars/TB" default:"7" testDefault:"45"`
	Segment   string `help:"price user should pay for segments stored on network per month in dollars/segment" default:"0.0000088" testDefault:"0.0000022"`
}

// ToModel returns the payments.ProjectUsagePriceModel representation of the project usage price.
func (p ProjectUsagePrice) ToModel() (model payments.ProjectUsagePriceModel, err error) {
	storageTBMonthDollars, err := decimal.NewFromString(p.StorageTB)
	if err != nil {
		return model, Error.Wrap(err)
	}
	egressTBDollars, err := decimal.NewFromString(p.EgressTB)
	if err != nil {
		return model, Error.Wrap(err)
	}
	segmentMonthDollars, err := decimal.NewFromString(p.Segment)
	if err != nil {
		return model, Error.Wrap(err)
	}

	// Shift is to change the precision from TB dollars to MB cents
	return payments.ProjectUsagePriceModel{
		StorageMBMonthCents: storageTBMonthDollars.Shift(-6).Shift(2),
		EgressMBCents:       egressTBDollars.Shift(-6).Shift(2),
		SegmentMonthCents:   segmentMonthDollars.Shift(2),
	}, nil
}

// Ensure that ProjectUsagePriceOverrides implements pflag.Value.
var _ pflag.Value = (*ProjectUsagePriceOverrides)(nil)

// ProjectUsagePriceOverrides represents a mapping between partners and project usage price overrides.
type ProjectUsagePriceOverrides struct {
	overrideMap map[string]ProjectUsagePrice
}

// Type returns the type of the pflag.Value.
func (ProjectUsagePriceOverrides) Type() string { return "paymentsconfig.ProjectUsagePriceOverrides" }

// String returns the string representation of the price overrides.
func (p *ProjectUsagePriceOverrides) String() string {
	if p == nil {
		return ""
	}
	var s strings.Builder
	left := len(p.overrideMap)
	for partner, prices := range p.overrideMap {
		s.WriteString(fmt.Sprintf("%s:%s,%s,%s", partner, prices.StorageTB, prices.EgressTB, prices.Segment))
		left--
		if left > 0 {
			s.WriteRune(';')
		}
	}
	return s.String()
}

// Set sets the list of price overrides to the parsed string.
func (p *ProjectUsagePriceOverrides) Set(s string) error {
	overrideMap := make(map[string]ProjectUsagePrice)
	for _, overrideStr := range strings.Split(s, ";") {
		if overrideStr == "" {
			continue
		}

		info := strings.Split(overrideStr, ":")
		if len(info) != 2 {
			return Error.New("Invalid price override (expected format partner:storage,egress,segment, got %s)", overrideStr)
		}

		partner := strings.TrimSpace(info[0])
		if len(partner) == 0 {
			return Error.New("Price override partner must not be empty")
		}

		pricesStr := info[1]
		prices := strings.Split(pricesStr, ",")
		if len(prices) != 3 {
			return Error.New("Invalid prices (expected format storage,egress,segment, got %s)", pricesStr)
		}

		for _, price := range prices {
			if _, err := decimal.NewFromString(price); err != nil {
				return Error.New("Invalid price (%s)", err)
			}
		}

		overrideMap[info[0]] = ProjectUsagePrice{
			StorageTB: prices[0],
			EgressTB:  prices[1],
			Segment:   prices[2],
		}
	}
	p.overrideMap = overrideMap
	return nil
}

// SetMap sets the internal mapping between partners and project usage prices.
func (p *ProjectUsagePriceOverrides) SetMap(overrides map[string]ProjectUsagePrice) {
	p.overrideMap = overrides
}

// ToModels returns the price overrides represented as a mapping between partners and project usage price models.
func (p ProjectUsagePriceOverrides) ToModels() (map[string]payments.ProjectUsagePriceModel, error) {
	models := make(map[string]payments.ProjectUsagePriceModel)
	for partner, prices := range p.overrideMap {
		model, err := prices.ToModel()
		if err != nil {
			return nil, err
		}
		models[partner] = model
	}
	return models, nil
}

// PackagePlans contains one time prices for partners.
type PackagePlans struct {
	Packages map[string]payments.PackagePlan
}

// Type returns the type of the pflag.Value.
func (PackagePlans) Type() string { return "paymentsconfig.PackagePlans" }

// String returns the string representation of the package plans.
func (p *PackagePlans) String() string {
	if p == nil {
		return ""
	}
	var s strings.Builder
	left := len(p.Packages)
	for partner, pkg := range p.Packages {
		s.WriteString(fmt.Sprintf("%s:%d,%d", partner, pkg.Price, pkg.Credit))
		left--
		if left > 0 {
			s.WriteRune(';')
		}
	}
	return s.String()
}

// Set sets the list of pricing plans to the parsed string.
func (p *PackagePlans) Set(s string) error {
	packages := make(map[string]payments.PackagePlan)
	for _, packagePlansStr := range strings.Split(s, ";") {
		if packagePlansStr == "" {
			continue
		}

		info := strings.Split(packagePlansStr, ":")
		if len(info) != 2 {
			return Error.New("Invalid package plan (expected format partner:price,credit got %s)", packagePlansStr)
		}

		partner := strings.TrimSpace(info[0])
		if len(partner) == 0 {
			return Error.New("Package plan partner must not be empty")
		}

		packageStr := info[1]
		pkg := strings.Split(packageStr, ",")
		if len(pkg) != 2 || pkg[0] == "" {
			return Error.New("Invalid package (expected format price,credit got %s)", packageStr)
		}

		if _, err := decimal.NewFromString(pkg[1]); err != nil {
			return Error.New("Invalid price (%s)", err)
		}

		priceCents, err := strconv.Atoi(pkg[0])
		if err != nil {
			return Error.Wrap(err)
		}

		creditCents, err := strconv.Atoi(pkg[1])
		if err != nil {
			return Error.Wrap(err)
		}

		packages[info[0]] = payments.PackagePlan{
			Price:  int64(priceCents),
			Credit: int64(creditCents),
		}
	}
	p.Packages = packages
	return nil
}

// Get a package plan by user agent.
func (p *PackagePlans) Get(userAgent []byte) (pkg payments.PackagePlan, err error) {
	entries, err := useragent.ParseEntries(userAgent)
	if err != nil {
		return payments.PackagePlan{}, Error.Wrap(err)
	}
	for _, entry := range entries {
		if pkg, ok := p.Packages[entry.Product]; ok {
			return pkg, nil
		}
	}
	return payments.PackagePlan{}, errs.New("no matching partner for (%s)", userAgent)
}
