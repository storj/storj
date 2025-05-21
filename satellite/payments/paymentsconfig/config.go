// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/useragent"
	"storj.io/storj/satellite/nodeselection"
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
	MinimumCharge      MinimumChargeConfig

	// TODO: if we decide to put default product in here and change away from overrides, change the type name.
	Products                        ProductPriceOverrides       `help:"a JSON mapping of non-zero product IDs to their names and price structures in the format: {\"productID\": {\"name\": \"...\", \"storage\": \"...\", \"egress\": \"...\", \"segment\": \"...\", \"egress_discount_ratio\": \"...\"}}. The egress discount ratio is the ratio of free egress per unit-month of storage"`
	PlacementPriceOverrides         PlacementProductMap         `help:"a JSON mapping of product ID to placements in the format: {productID0:[placement0],productID1:[placement1,placement2]}. Products must be defined by the --payments.products config, or the satellite will not start."`
	PartnersPlacementPriceOverrides PartnersPlacementProductMap `help:"a JSON mapping of partners to a mapping of product ID to placements in the format: {\"partner0\":{productID0:[placement0],productID1:[placement1,placement2]}}. If a partner uses a placement not defined for them in this config, they will be charged according to --payments.placement-price-overrides."`

	BonusRate           int64          `help:"amount of percents that user will earn as bonus credits by depositing in STORJ tokens" default:"10"`
	UsagePriceOverrides PriceOverrides `help:"semicolon-separated usage price overrides in the format partner:storage,egress,segment,egress_discount_ratio. The egress discount ratio is the ratio of free egress per unit-month of storage"`
	PackagePlans        PackagePlans   `help:"semicolon-separated partner package plans in the format partner:price,credit. Price and credit are in cents USD."`
}

// MinimumChargeConfig holds the configuration for minimum charge enforcement.
type MinimumChargeConfig struct {
	// Amount specifies the minimum amount in cents that will be charged to a customer for an invoice.
	// If an invoice total is below this amount, an additional line item will be added to reach this threshold.
	// Set to 0 to disable minimum charge enforcement.
	Amount             int64  `help:"minimum amount in cents to charge customers per invoice period (0 to disable)" default:"0"`
	NewUsersCutoffDate string `help:"date after which newly-created users will have minimum charges applied (YYYY-MM-DD, empty for no effect)" default:""`
	AllUsersDate       string `help:"date after which all users will have minimum charges applied (YYYY-MM-DD, empty for no effect)" default:""`
}

// GetNewUsersCutoffDate returns the date after which newly-created users will have minimum charges applied.
// If the date is not set, it returns nil.
func (p MinimumChargeConfig) GetNewUsersCutoffDate() (*time.Time, error) {
	if p.NewUsersCutoffDate == "" {
		return nil, nil
	}

	date, err := time.Parse("2006-01-02", p.NewUsersCutoffDate)
	if err != nil {
		return nil, err
	}

	return &date, nil
}

// GetAllUsersDate returns the date after which all users will have minimum charges applied.
// If the date is not set, it returns nil.
func (p MinimumChargeConfig) GetAllUsersDate() (*time.Time, error) {
	if p.AllUsersDate == "" {
		return nil, nil
	}

	date, err := time.Parse("2006-01-02", p.AllUsersDate)
	if err != nil {
		return nil, err
	}

	return &date, nil
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
	Name string
	ProjectUsagePrice
}

// ProductPriceOverrides represents a mapping between a string and product price overrides.
type ProductPriceOverrides struct {
	overrideMap map[int32]ProductUsagePrice
}

// Type returns the type of the pflag.Value.
func (ProductPriceOverrides) Type() string { return "paymentsconfig.ProductPriceOverrides" }

type productUsagePriceJson struct {
	Name                string `json:"name"`
	Storage             string `json:"storage"`
	Egress              string `json:"egress"`
	Segment             string `json:"segment"`
	EgressDiscountRatio string `json:"egress_discount_ratio"`
}

// String returns the JSON string representation of the price overrides.
func (p *ProductPriceOverrides) String() string {
	if p == nil || len(p.overrideMap) == 0 {
		return ""
	}

	pricesConv := make(map[int32]productUsagePriceJson)
	for i, price := range p.overrideMap {
		pricesConv[i] = productUsagePriceJson{
			Name:                price.Name,
			Storage:             price.StorageTB,
			Egress:              price.EgressTB,
			Segment:             price.Segment,
			EgressDiscountRatio: fmt.Sprintf("%.2f", price.EgressDiscountRatio),
		}
	}
	prices, err := json.Marshal(pricesConv)
	if err != nil {
		return ""
	}

	return string(prices)
}

// Set sets the list of price overrides to the JSON string.
func (p *ProductPriceOverrides) Set(s string) error {
	if s == "" {
		return nil
	}

	pricesConv := make(map[int32]productUsagePriceJson)
	err := json.Unmarshal([]byte(s), &pricesConv)
	if err != nil {
		return err
	}
	prices := make(map[int32]ProductUsagePrice)
	for id, price := range pricesConv {
		if id == 0 {
			return Error.New("Product ID must not be 0")
		}
		if price.Name == "" {
			return Error.New("Product Name must not be empty")
		}
		egressDiscount, err := strconv.ParseFloat(price.EgressDiscountRatio, 64)
		if err != nil {
			return Error.New("Invalid egress discount ratio '%s' (%s)", price.EgressDiscountRatio, err)
		}
		prices[id] = ProductUsagePrice{
			Name: price.Name,
			ProjectUsagePrice: ProjectUsagePrice{
				StorageTB:           price.Storage,
				EgressTB:            price.Egress,
				Segment:             price.Segment,
				EgressDiscountRatio: egressDiscount,
			},
		}
	}
	p.overrideMap = prices
	return nil
}

// SetMap sets the internal mapping between a product ID and usage prices.
func (p *ProductPriceOverrides) SetMap(overrides map[int32]ProductUsagePrice) {
	p.overrideMap = overrides
}

// ToModels returns the price overrides represented as a mapping between a string and product usage price models.
func (p ProductPriceOverrides) ToModels() (map[int32]payments.ProductUsagePriceModel, error) {
	models := make(map[int32]payments.ProductUsagePriceModel)
	for key, prices := range p.overrideMap {
		projectUsageModel, err := prices.ToModel()
		if err != nil {
			return nil, err
		}
		models[key] = payments.ProductUsagePriceModel{
			ProductID:              key,
			ProductName:            prices.Name,
			ProjectUsagePriceModel: projectUsageModel,
		}
	}
	return models, nil
}

// Get returns the product usage price for the given product ID.
func (p *ProductPriceOverrides) Get(id int32) (prices ProductUsagePrice, ok bool) {
	if p == nil {
		return ProductUsagePrice{}, false
	}
	prices, ok = p.overrideMap[id]
	return prices, ok
}

// Ensure that PlacementProductMap implements pflag.Value.
var _ pflag.Value = (*PlacementProductMap)(nil)

// PlacementProductMap maps placements to products.
type PlacementProductMap struct {
	placementProductMap map[int]int32
}

// SetMap sets the internal mapping between placements and products.
func (p *PlacementProductMap) SetMap(placementProductMap map[int]int32) {
	p.placementProductMap = placementProductMap
}

// ToMap flattens the placement to product map typed as payments.PlacementProductIdMap.
func (p *PlacementProductMap) ToMap() payments.PlacementProductIdMap {
	return p.placementProductMap
}

// Type returns the type of the pflag.Value.
func (PlacementProductMap) Type() string { return "paymentsconfig.PlacementProductIdMap" }

// String returns the JSON string representation of the placements to product map.
func (p *PlacementProductMap) String() string {
	if p == nil || len(p.placementProductMap) == 0 {
		return ""
	}

	productToPlacements := make(map[int32][]int)
	for placement, product := range p.placementProductMap {
		productToPlacements[product] = append(productToPlacements[product], placement)
	}

	data, err := json.Marshal(productToPlacements)
	if err != nil {
		return ""
	}
	return string(data)
}

// Set sets the placement to product mappings to the JSON string.
func (p *PlacementProductMap) Set(s string) error {
	if s == "" {
		return nil
	}

	mapping := make(map[int32][]int)
	err := json.Unmarshal([]byte(s), &mapping)
	if err != nil {
		return err
	}

	placementProductMap := make(map[int]int32)
	for productID, placements := range mapping {
		for _, placement := range placements {
			placementProductMap[placement] = productID
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

// String returns the JSON string representation of the partners to placements to product map.
func (p *PartnersPlacementProductMap) String() string {
	if p == nil || len(p.partnerPlacementProductMap) == 0 {
		return ""
	}

	mapping := make(map[string]map[int32][]int)
	for partner, placementProduct := range p.partnerPlacementProductMap {
		productPlacements := make(map[int32][]int)
		for placement, product := range placementProduct.placementProductMap {
			productPlacements[product] = append(productPlacements[product], placement)
		}
		mapping[partner] = productPlacements
	}

	data, err := json.Marshal(mapping)
	if err != nil {
		return ""
	}
	return string(data)
}

// Set sets the partners placements to products mappings to the JSON string.
func (p *PartnersPlacementProductMap) Set(s string) error {
	if s == "" {
		return nil
	}

	var mapping map[string]map[int32][]int
	err := json.Unmarshal([]byte(s), &mapping)
	if err != nil {
		return err
	}
	partnerPlacementProductMap := make(map[string]PlacementProductMap)
	for partner, productPlacement := range mapping {
		placementProductMap := PlacementProductMap{
			placementProductMap: make(map[int]int32),
		}
		for product, placements := range productPlacement {
			for _, placement := range placements {
				placementProductMap.placementProductMap[placement] = product
			}
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

// ValidatePlacementOverrideMap ensures placements and product IDs in price override maps exist.
func ValidatePlacementOverrideMap(overrideMap map[int]int32, productPrices map[int32]payments.ProductUsagePriceModel, placements nodeselection.PlacementDefinitions) error {
	for placement, productID := range overrideMap {
		if _, ok := placements[storj.PlacementConstraint(placement)]; !ok {
			return errs.New("placement %d not found in placement constraints", placement)
		}

		if _, ok := productPrices[productID]; !ok {
			return errs.New("product %d not found in product prices", productID)
		}
	}
	return nil
}
