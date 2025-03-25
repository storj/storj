// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"

	"storj.io/common/useragent"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/payments/stripe"
)

// Error is payments config err class.
var Error = errs.Class("payments config")

// Config defines global payments config.
type Config struct {
	Provider     string        `help:"payments provider to use" default:""`
	MockProvider stripe.Client `internal:"true"`

	BillingConfig      billing.Config
	StripeCoinPayments stripe.Config
	Storjscan          storjscan.Config
	UsagePrice         ProjectUsagePrice

	// TODO: if we decide to put default product in here and change away from overrides, change the type name.
	Products                        ProductPriceOverrides       `help:"semicolon-separated list of products and their product IDs and price structures in the format: product:productID,storage,egress,segment,egress_discount_ratio. The egress discount ratio is the ratio of free egress per unit-month of storage"`
	PlacementPriceOverrides         PlacementProductMap         `help:"semicolon-separated list of placement price overrides in the format: placement:product. Multiple placements may be mapped to a single product like so: p0,p1:product. Products must be defined by the --payments.products config, or the satellite will not start."`
	PartnersPlacementPriceOverrides PartnersPlacementProductMap `help:"semicolon-separated list of partners to placement price overrides in the format: partner0[placement0:product0;placement1,placement2:product1];partner1[etc...] If a partner uses a placement not defined for them in this config, they will be charged according to --payments.placement-price-overrides."`

	BonusRate           int64          `help:"amount of percents that user will earn as bonus credits by depositing in STORJ tokens" default:"10"`
	UsagePriceOverrides PriceOverrides `help:"semicolon-separated usage price overrides in the format partner:storage,egress,segment,egress_discount_ratio. The egress discount ratio is the ratio of free egress per unit-month of storage"`
	PackagePlans        PackagePlans   `help:"semicolon-separated partner package plans in the format partner:price,credit. Price and credit are in cents USD."`
}

// ProjectUsagePrice holds the configuration for the satellite's project usage price model.
type ProjectUsagePrice struct {
	StorageTB           string  `help:"price user should pay for storage per month in dollars/TB" default:"4" testDefault:"10"`
	EgressTB            string  `help:"price user should pay for egress in dollars/TB" default:"7" testDefault:"45"`
	Segment             string  `help:"price user should pay for segments stored on network per month in dollars/segment" default:"0.0000088" testDefault:"0.0000022"`
	EgressDiscountRatio float64 `internal:"true"`
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
		EgressDiscountRatio: p.EgressDiscountRatio,
	}, nil
}

// Ensure that PriceOverrides implements pflag.Value.
var _ pflag.Value = (*PriceOverrides)(nil)

// PriceOverrides represents a mapping between a string and price overrides.
type PriceOverrides struct {
	overrideMap map[string]ProjectUsagePrice
}

// Type returns the type of the pflag.Value.
func (PriceOverrides) Type() string { return "paymentsconfig.PriceOverrides" }

// String returns the string representation of the price overrides.
func (p *PriceOverrides) String() string {
	if p == nil {
		return ""
	}
	var s strings.Builder
	left := len(p.overrideMap)
	for key, prices := range p.overrideMap {
		egressDiscount := strconv.FormatFloat(prices.EgressDiscountRatio, 'f', -1, 64)
		s.WriteString(fmt.Sprintf("%s:%s,%s,%s,%s", key, prices.StorageTB, prices.EgressTB, prices.Segment, egressDiscount))
		left--
		if left > 0 {
			s.WriteRune(';')
		}
	}
	return s.String()
}

// Set sets the list of price overrides to the parsed string.
func (p *PriceOverrides) Set(s string) error {
	overrideMap := make(map[string]ProjectUsagePrice)
	for _, overrideStr := range strings.Split(s, ";") {
		if overrideStr == "" {
			continue
		}

		info := strings.Split(overrideStr, ":")
		if len(info) != 2 {
			return Error.New("Invalid price override (expected format key:storage,egress,segment, got %s)", overrideStr)
		}

		key := strings.TrimSpace(info[0])
		if len(key) == 0 {
			return Error.New("Price override key must not be empty")
		}

		valuesStr := info[1]
		values := strings.Split(valuesStr, ",")
		if len(values) != 4 {
			return Error.New("Invalid values (expected format storage,egress,segment,egress_discount_ratio, got %s)", valuesStr)
		}

		for i := 0; i < 3; i++ {
			if _, err := decimal.NewFromString(values[i]); err != nil {
				return Error.New("Invalid price '%s' (%s)", values[i], err)
			}
		}

		egressDiscount, err := strconv.ParseFloat(values[3], 64)
		if err != nil {
			return Error.New("Invalid egress discount ratio '%s' (%s)", values[3], err)
		}

		overrideMap[info[0]] = ProjectUsagePrice{
			StorageTB:           values[0],
			EgressTB:            values[1],
			Segment:             values[2],
			EgressDiscountRatio: egressDiscount,
		}
	}
	p.overrideMap = overrideMap
	return nil
}

// SetMap sets the internal mapping between a string and usage prices.
func (p *PriceOverrides) SetMap(overrides map[string]ProjectUsagePrice) {
	p.overrideMap = overrides
}

// ToModels returns the price overrides represented as a mapping between a string and usage price models.
func (p PriceOverrides) ToModels() (map[string]payments.ProjectUsagePriceModel, error) {
	models := make(map[string]payments.ProjectUsagePriceModel)
	for key, prices := range p.overrideMap {
		model, err := prices.ToModel()
		if err != nil {
			return nil, err
		}
		models[key] = model
	}
	return models, nil
}

// ProductUsagePrice represents the product ID and usage price for a product.
type ProductUsagePrice struct {
	ProductID int32
	ProjectUsagePrice
}

// ProductPriceOverrides represents a mapping between a string and product price overrides.
type ProductPriceOverrides struct {
	overrideMap map[string]ProductUsagePrice
}

// Type returns the type of the pflag.Value.
func (ProductPriceOverrides) Type() string { return "paymentsconfig.ProductPriceOverrides" }

// String returns the string representation of the price overrides.
func (p *ProductPriceOverrides) String() string {
	if p == nil {
		return ""
	}
	var s strings.Builder
	left := len(p.overrideMap)
	for key, prices := range p.overrideMap {
		egressDiscount := strconv.FormatFloat(prices.EgressDiscountRatio, 'f', -1, 64)
		s.WriteString(fmt.Sprintf("%s:%d,%s,%s,%s,%s", key, prices.ProductID, prices.StorageTB, prices.EgressTB, prices.Segment, egressDiscount))
		left--
		if left > 0 {
			s.WriteRune(';')
		}
	}
	return s.String()
}

// Set sets the list of price overrides to the parsed string.
func (p *ProductPriceOverrides) Set(s string) error {
	overrideMap := make(map[string]ProductUsagePrice)
	productIDsSeen := make(map[int32]struct{})
	for _, overrideStr := range strings.Split(s, ";") {
		if overrideStr == "" {
			continue
		}

		info := strings.Split(overrideStr, ":")
		if len(info) != 2 {
			return Error.New("Invalid price override (expected format key:productID,storage,egress,segment,egress_discount_ratio, got %s)", overrideStr)
		}

		key := strings.TrimSpace(info[0])
		if len(key) == 0 {
			return Error.New("Price override key must not be empty")
		}

		valuesStr := info[1]
		values := strings.Split(valuesStr, ",")
		if len(values) != 5 {
			return Error.New("Invalid values (expected format productID,storage,egress,segment,egress_discount_ratio, got %s)", valuesStr)
		}

		productID64, err := strconv.ParseInt(values[0], 10, 32)
		if err != nil {
			return Error.New("Invalid product ID '%s' (%s)", values[0], err)
		}
		productID := int32(productID64)
		if _, ok := productIDsSeen[productID]; ok {
			return Error.New("Product ID mapped multiple times. Check the config for (%d) and ensure only one product has this product ID.", productID)
		}
		productIDsSeen[productID] = struct{}{}

		for i := 1; i < 4; i++ {
			if _, err := decimal.NewFromString(values[i]); err != nil {
				return Error.New("Invalid price '%s' (%s)", values[i], err)
			}
		}

		egressDiscount, err := strconv.ParseFloat(values[4], 64)
		if err != nil {
			return Error.New("Invalid egress discount ratio '%s' (%s)", values[4], err)
		}

		overrideMap[info[0]] = ProductUsagePrice{
			ProductID: productID,
			ProjectUsagePrice: ProjectUsagePrice{
				StorageTB:           values[1],
				EgressTB:            values[2],
				Segment:             values[3],
				EgressDiscountRatio: egressDiscount,
			},
		}
	}
	p.overrideMap = overrideMap
	return nil
}

// SetMap sets the internal mapping between a string and usage prices.
func (p *ProductPriceOverrides) SetMap(overrides map[string]ProductUsagePrice) {
	p.overrideMap = overrides
}

// ToModels returns the price overrides represented as a mapping between a string and product usage price models.
func (p ProductPriceOverrides) ToModels() (map[string]payments.ProductUsagePriceModel, error) {
	models := make(map[string]payments.ProductUsagePriceModel)
	for key, prices := range p.overrideMap {
		projectUsageModel, err := prices.ToModel()
		if err != nil {
			return nil, err
		}
		models[key] = payments.ProductUsagePriceModel{
			ProductID:              prices.ProductID,
			ProjectUsagePriceModel: projectUsageModel,
		}
	}
	return models, nil
}

// Ensure that PlacementProductMap implements pflag.Value.
var _ pflag.Value = (*PlacementProductMap)(nil)

// PlacementProductMap maps placements to products.
type PlacementProductMap struct {
	placementProductMap map[int]string
}

// SetMap sets the internal mapping between placements and products.
func (p *PlacementProductMap) SetMap(placementProductMap map[int]string) {
	p.placementProductMap = placementProductMap
}

// ToMap flattens the placement to product map typed as payments.PlacementProductMap.
func (p *PlacementProductMap) ToMap() payments.PlacementProductMap {
	return p.placementProductMap
}

// Type returns the type of the pflag.Value.
func (PlacementProductMap) Type() string { return "paymentsconfig.PlacementProductMap" }

// String returns the string representation of the placements to product map. Placements of a single key-value pair
// are sorted and the greater key-value strings themselves are sorted in ascending order.
func (p *PlacementProductMap) String() string {
	if p == nil {
		return ""
	}

	productToPlacements := make(map[string][]string)
	for placement, product := range p.placementProductMap {
		placements := productToPlacements[product]
		productToPlacements[product] = append(placements, strconv.Itoa(placement))
	}

	var kvs []string
	for product, placements := range productToPlacements {
		sort.Strings(placements)
		kvs = append(kvs, fmt.Sprintf("%s:%s", strings.Join(placements, ","), product))
	}

	sort.Strings(kvs)

	return strings.Join(kvs, ";")
}

// Set sets the placement to product mappings to the parsed string.
func (p *PlacementProductMap) Set(s string) error {
	placementProductMap := make(map[int]string)
	productsSeen := make(map[string]struct{})
	for _, placementsToProductStr := range strings.Split(s, ";") {
		if placementsToProductStr == "" {
			continue
		}

		info := strings.Split(placementsToProductStr, ":")
		if len(info) != 2 {
			return Error.New("Invalid placements to product string (expected format p0,p1:product, got %s)", placementsToProductStr)
		}

		placementsStr := strings.TrimSpace(info[0])
		placements := strings.Split(placementsStr, ",")
		if len(placements) == 0 {
			return Error.New("Placements must not be empty")
		}

		product := info[1]

		if _, ok := productsSeen[product]; ok {
			return Error.New("Product mapped multiple times. Check the config for (%s) and ensure only a single key-value pair exists per product.", product)
		}
		productsSeen[product] = struct{}{}

		for _, p := range placements {
			pInt, err := strconv.Atoi(p)
			if err != nil {
				return Error.New("Placement must be an int: %w", err)
			}
			if _, ok := placementProductMap[pInt]; ok {
				return Error.New("Placements cannot be mapped to multiple products. Check the config for placement %d", pInt)
			}
			placementProductMap[pInt] = product
		}
	}
	p.placementProductMap = placementProductMap
	return nil
}

// Ensure that PartnersPlacementProductMap implements pflag.Value.
var _ pflag.Value = (*PartnersPlacementProductMap)(nil)

// PartnersPlacementProductMap maps partners to placements to products map.
type PartnersPlacementProductMap struct {
	partnerPlacementProductMap map[string]PlacementProductMap
}

// SetMap sets the internal mapping between partners, placements and products.
func (p *PartnersPlacementProductMap) SetMap(partnerPlacementProductMap map[string]PlacementProductMap) {
	p.partnerPlacementProductMap = partnerPlacementProductMap
}

// ToMap flattens the partners to placements to product map
// typed as payments.PartnersPlacementProductMap.
func (p *PartnersPlacementProductMap) ToMap() payments.PartnersPlacementProductMap {
	productMap := make(payments.PartnersPlacementProductMap)
	for partner, placementProductMap := range p.partnerPlacementProductMap {
		productMap[partner] = placementProductMap.ToMap()
	}
	return productMap
}

// Type returns the type of the pflag.Value.
func (PartnersPlacementProductMap) Type() string { return "paymentsconfig.PartnersPlacementProductMap" }

// String returns the string representation of the partners to placements to product map. Partner configs are
// sorted in ascending order by partner name.
func (p *PartnersPlacementProductMap) String() string {
	if p == nil {
		return ""
	}

	var kvs []string
	for partner, placementProductMap := range p.partnerPlacementProductMap {
		kvs = append(kvs, fmt.Sprintf("%s[%s]", partner, placementProductMap.String()))
	}

	sort.Strings(kvs)

	return strings.Join(kvs, ";")
}

// Set sets the partners placements to products mappings to the parsed string.
func (p *PartnersPlacementProductMap) Set(s string) error {
	partnerPlacementProductMap := make(map[string]PlacementProductMap)

	if strings.HasSuffix(s, "]") && !strings.HasSuffix(s, ";") {
		s += ";"
	}
	for _, partnerConfig := range strings.Split(s, "];") {
		if partnerConfig == "" {
			continue
		}

		info := strings.Split(partnerConfig, "[")
		if len(info) != 2 {
			return Error.New("Invalid partner placements to product string (expected format partner[placement0,placement1:product], got %s)", partnerConfig)
		}

		partner := strings.TrimSpace(info[0])
		if _, ok := partnerPlacementProductMap[partner]; ok {
			return Error.New("Partner's placements to products mapping was defined more than once. Check the config for partner %s", partner)
		}

		var placementProductMap PlacementProductMap
		err := placementProductMap.Set(info[1])
		if err != nil {
			return err
		}
		partnerPlacementProductMap[partner] = placementProductMap
	}
	p.partnerPlacementProductMap = partnerPlacementProductMap
	return nil
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
