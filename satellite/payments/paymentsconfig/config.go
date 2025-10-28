// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"gopkg.in/yaml.v3"

	"storj.io/common/memory"
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

	DeleteProjectCostThreshold int64 `help:"the amount of usage in cents above which a project's usage should be paid before allowing deletion. Set to 0 to disable the threshold." default:"0"`

	// TODO: if we decide to put default product in here and change away from overrides, change the type name.
	Products                        ProductPriceOverrides       `help:"a YAML list of products with their price structures. See satellite/payments/paymentsconfig/README.md for more details."`
	PlacementPriceOverrides         PlacementProductMap         `help:"a YAML mapping of product ID to placements. See satellite/payments/paymentsconfig/README.md for more details."`
	PartnersPlacementPriceOverrides PartnersPlacementProductMap `help:"a YAML mapping of partners to a mapping of product ID to placements. See satellite/payments/paymentsconfig/README.md for more details."`

	BonusRate           int64          `help:"amount of percents that user will earn as bonus credits by depositing in STORJ tokens" default:"10"`
	UsagePriceOverrides PriceOverrides `help:"semicolon-separated usage price overrides in the format partner:storage,egress,segment,egress_discount_ratio. The egress discount ratio is the ratio of free egress per unit-month of storage"`
	PackagePlans        PackagePlans   `help:"semicolon-separated partner package plans in the format partner:price,credit. Price and credit are in cents USD."`
}

// MinimumChargeConfig holds the configuration for minimum charge enforcement.
type MinimumChargeConfig struct {
	// Amount specifies the minimum amount in cents that will be charged to a customer for an invoice.
	// If an invoice total is below this amount, an additional line item will be added to reach this threshold.
	// Set to 0 to disable minimum charge enforcement.
	Amount        int64  `help:"minimum amount in cents to charge customers per invoice period (0 to disable)" default:"0"`
	EffectiveDate string `help:"date after which all users will have minimum charges applied (YYYY-MM-DD), empty to apply immediately" default:""`
}

// GetEffectiveDate returns the date after which all users will have minimum charges applied.
// If the date is not set, it returns nil.
func (p MinimumChargeConfig) GetEffectiveDate() (*time.Time, error) {
	if p.EffectiveDate == "" {
		return nil, nil
	}

	date, err := time.Parse("2006-01-02", p.EffectiveDate)
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

	var segmentMonthCents decimal.Decimal
	if p.Segment != "" {
		segmentMonthDollars, err := decimal.NewFromString(p.Segment)
		if err != nil {
			return model, Error.Wrap(err)
		}
		segmentMonthCents = segmentMonthDollars.Shift(2)
	}

	// Shift is to change the precision from TB dollars to MB cents
	return payments.ProjectUsagePriceModel{
		StorageMBMonthCents: storageTBMonthDollars.Shift(-6).Shift(2),
		EgressMBCents:       egressTBDollars.Shift(-6).Shift(2),
		SegmentMonthCents:   segmentMonthCents,
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
func (*PriceOverrides) Type() string { return "paymentsconfig.PriceOverrides" }

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
func (p *PriceOverrides) ToModels() (map[string]payments.ProjectUsagePriceModel, error) {
	if p == nil {
		return nil, errs.New("no price overrides defined")
	}

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

// ProductUsagePrice represents the product name, SKUs and usage price for a product.
type ProductUsagePrice struct {
	ID                     int32
	Name                   string
	ShortName              string
	SmallObjectFee         string
	MinimumRetentionFee    string
	SmallObjectFeeSKU      string
	MinimumRetentionFeeSKU string
	EgressOverageMode      bool
	IncludedEgressSKU      string
	StorageRemainder       string
	// PriceSummary will be displayed on the Pro Account info card in the UI.
	PriceSummary string
	UseGBUnits   bool
	ProductSKUs
	ProjectUsagePrice
}

// ProductSKUs holds the SKUs for a product's storage, egress, and segment usage.
type ProductSKUs struct {
	StorageSKU string `json:"storageSKU"`
	EgressSKU  string `json:"egressSKU"`
	SegmentSKU string `json:"segmentSKU"`
}

// ProductPriceOverrides represents a mapping between a string and product price overrides.
type ProductPriceOverrides []ProductUsagePrice

// Type returns the type of the pflag.Value.
func (*ProductPriceOverrides) Type() string { return "paymentsconfig.ProductPriceOverrides" }

// ProductUsagePriceYaml represents the YAML representation of a product usage price.
// Exported for testing purposes.
type ProductUsagePriceYaml struct {
	ID                     int32  `yaml:"id" json:"id"`
	Name                   string `yaml:"name" json:"name"`
	ShortName              string `yaml:"short-name" json:"short_name"`
	Storage                string `yaml:"storage" json:"storage"`
	StorageSKU             string `yaml:"storage-sku" json:"storage_sku"`
	Egress                 string `yaml:"egress" json:"egress"`
	EgressSKU              string `yaml:"egress-sku" json:"egress_sku"`
	Segment                string `yaml:"segment" json:"segment"`
	SegmentSKU             string `yaml:"segment-sku" json:"segment_sku"`
	EgressDiscountRatio    string `yaml:"egress-discount-ratio" json:"egress_discount_ratio"`
	SmallObjectFee         string `yaml:"small-object-fee" json:"-"`
	MinimumRetentionFee    string `yaml:"minimum-retention-fee" json:"-"`
	SmallObjectFeeSKU      string `yaml:"small-object-fee-sku" json:"-"`
	MinimumRetentionFeeSKU string `yaml:"minimum-retention-fee-sku" json:"-"`
	EgressOverageMode      bool   `yaml:"egress-overage-mode" json:"-"`
	IncludedEgressSKU      string `yaml:"included-egress-sku" json:"-"`
	StorageRemainder       string `yaml:"storage-remainder" json:"-"`
	// PriceSummary will be displayed on the Pro Account info card in the UI.
	PriceSummary string `yaml:"price-summary" json:"-"`
	UseGBUnits   bool   `yaml:"use-gb-units" json:"-"`
}

// String returns the YAML string representation of the price overrides.
func (p *ProductPriceOverrides) String() string {
	if p == nil || len(*p) == 0 {
		return ""
	}

	pricesConv := make([]ProductUsagePriceYaml, len(*p))
	for i, price := range *p {
		pricesConv[i] = ProductUsagePriceYaml{
			ID:                     price.ID,
			Name:                   price.Name,
			ShortName:              price.ShortName,
			Storage:                price.StorageTB,
			StorageSKU:             price.StorageSKU,
			Egress:                 price.EgressTB,
			EgressSKU:              price.EgressSKU,
			Segment:                price.Segment,
			SegmentSKU:             price.SegmentSKU,
			EgressDiscountRatio:    fmt.Sprintf("%.2f", price.EgressDiscountRatio),
			SmallObjectFee:         price.SmallObjectFee,
			MinimumRetentionFee:    price.MinimumRetentionFee,
			SmallObjectFeeSKU:      price.SmallObjectFeeSKU,
			MinimumRetentionFeeSKU: price.MinimumRetentionFeeSKU,
			EgressOverageMode:      price.EgressOverageMode,
			IncludedEgressSKU:      price.IncludedEgressSKU,
			StorageRemainder:       price.StorageRemainder,
			UseGBUnits:             price.UseGBUnits,
		}
	}
	prices, err := yaml.Marshal(pricesConv)
	if err != nil {
		return ""
	}

	return string(prices)
}

// Set sets the list of price overrides to the YAML string.
func (p *ProductPriceOverrides) Set(s string) error {
	if s == "" {
		return nil
	}

	s = strings.TrimSpace(s)
	strBytes := []byte(s)
	var pricesConv []ProductUsagePriceYaml
	switch {
	case strings.HasSuffix(s, ".yaml"):
		// YAML file path
		data, err := os.ReadFile(s)
		if err != nil {
			return errs.New("Couldn't read product config file from %s: %v", s, err)
		}

		err = yaml.Unmarshal(data, &pricesConv)
		if err != nil {
			return errs.New("failed to parse product config YAML file: %v", err)
		}
	default:
		// YAML string
		err := yaml.Unmarshal(strBytes, &pricesConv)
		if err != nil {
			return errs.New("failed to parse product config YAML: %v", err)
		}
	}
	prices := make(ProductPriceOverrides, len(pricesConv))
	for i, price := range pricesConv {
		if price.ID == 0 {
			return Error.New("Product ID must not be 0")
		}
		if price.Name == "" {
			return Error.New("Product Name must not be empty")
		}

		var err error
		var egressDiscount float64
		if price.EgressDiscountRatio != "" {
			egressDiscount, err = strconv.ParseFloat(price.EgressDiscountRatio, 64)
			if err != nil {
				return Error.New("Invalid egress discount ratio '%s' (%s)", price.EgressDiscountRatio, err)
			}
		}
		prices[i] = ProductUsagePrice{
			ID:        price.ID,
			Name:      price.Name,
			ShortName: price.ShortName,
			ProductSKUs: ProductSKUs{
				StorageSKU: price.StorageSKU,
				EgressSKU:  price.EgressSKU,
				SegmentSKU: price.SegmentSKU,
			},
			ProjectUsagePrice: ProjectUsagePrice{
				StorageTB:           price.Storage,
				EgressTB:            price.Egress,
				Segment:             price.Segment,
				EgressDiscountRatio: egressDiscount,
			},
			SmallObjectFee:         price.SmallObjectFee,
			MinimumRetentionFee:    price.MinimumRetentionFee,
			SmallObjectFeeSKU:      price.SmallObjectFeeSKU,
			MinimumRetentionFeeSKU: price.MinimumRetentionFeeSKU,
			EgressOverageMode:      price.EgressOverageMode,
			IncludedEgressSKU:      price.IncludedEgressSKU,
			StorageRemainder:       price.StorageRemainder,
			PriceSummary:           price.PriceSummary,
			UseGBUnits:             price.UseGBUnits,
		}
	}
	*p = prices
	return nil
}

// SetMap sets the internal mapping between a product ID and usage prices.
func (p *ProductPriceOverrides) SetMap(overrides map[int32]ProductUsagePrice) {
	productPrices := make([]ProductUsagePrice, 0, len(overrides))
	for id, price := range overrides {
		price.ID = id
		productPrices = append(productPrices, price)
	}
	*p = productPrices
}

// ToModels returns the price overrides represented as a mapping between a string and product usage price models.
func (p *ProductPriceOverrides) ToModels() (map[int32]payments.ProductUsagePriceModel, error) {
	if p == nil {
		return nil, errs.New("no product prices defined")
	}

	models := make(map[int32]payments.ProductUsagePriceModel)
	for _, prices := range *p {
		projectUsageModel, err := prices.ToModel()
		if err != nil {
			return nil, err
		}

		smallObjectFee := decimal.Zero
		if prices.SmallObjectFee != "" {
			smallObjectFee, err = decimal.NewFromString(prices.SmallObjectFee)
			if err != nil {
				return nil, Error.Wrap(err)
			}
		}
		minimumRetentionFee := decimal.Zero
		if prices.MinimumRetentionFee != "" {
			minimumRetentionFee, err = decimal.NewFromString(prices.MinimumRetentionFee)
			if err != nil {
				return nil, Error.Wrap(err)
			}
		}

		var storageRemainderBytes int64
		if prices.StorageRemainder != "" {
			var storageRemainder memory.Size
			err = storageRemainder.Set(prices.StorageRemainder)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			storageRemainderBytes = storageRemainder.Int64()
		}

		models[prices.ID] = payments.ProductUsagePriceModel{
			ProductID:                prices.ID,
			ProductName:              prices.Name,
			ProductShortName:         prices.ShortName,
			StorageSKU:               prices.StorageSKU,
			EgressSKU:                prices.EgressSKU,
			SegmentSKU:               prices.SegmentSKU,
			ProjectUsagePriceModel:   projectUsageModel,
			SmallObjectFeeCents:      smallObjectFee.Shift(-6).Shift(2),
			MinimumRetentionFeeCents: minimumRetentionFee.Shift(-6).Shift(2),
			SmallObjectFeeSKU:        prices.SmallObjectFeeSKU,
			MinimumRetentionFeeSKU:   prices.MinimumRetentionFeeSKU,
			EgressOverageMode:        prices.EgressOverageMode,
			IncludedEgressSKU:        prices.IncludedEgressSKU,
			StorageRemainderBytes:    storageRemainderBytes,
			UseGBUnits:               prices.UseGBUnits,
			PriceSummary:             prices.PriceSummary,
		}
	}
	return models, nil
}

// Get returns the product usage price for the given product ID.
func (p *ProductPriceOverrides) Get(id int32) (prices ProductUsagePrice, ok bool) {
	if p == nil {
		return ProductUsagePrice{}, false
	}
	for _, price := range *p {
		if price.ID == id {
			return price, true
		}
	}
	return ProductUsagePrice{}, false
}

// PlacementOverrides holds the placement overrides for products and partners defined in a common file.
// This format is to be used if placement override configurations are to be defined in a single file.
// Exported for testing purposes.
type PlacementOverrides struct {
	ProductPlacements        map[int32][]int            `yaml:"product-placements"`
	PartnerProductPlacements map[string]map[int32][]int `yaml:"partner-product-placements"`
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

// String returns the YAML string representation of the placements to product map.
func (p *PlacementProductMap) String() string {
	if p == nil || len(p.placementProductMap) == 0 {
		return ""
	}

	productToPlacements := make(map[int32][]int)
	for placement, product := range p.placementProductMap {
		productToPlacements[product] = append(productToPlacements[product], placement)
	}

	data, err := yaml.Marshal(productToPlacements)
	if err != nil {
		return ""
	}
	return string(data)
}

// Set sets the placement to product mappings to the YAML string.
func (p *PlacementProductMap) Set(s string) error {
	if s == "" {
		return nil
	}

	s = strings.TrimSpace(s)
	strBytes := []byte(s)

	var placementProducts map[int32][]int
	switch {
	case strings.HasSuffix(s, ".yaml"):
		// YAML file path
		data, err := os.ReadFile(s)
		if err != nil {
			return errs.New("Couldn't read product config file from %s: %v", s, err)
		}

		var overrides PlacementOverrides
		err = yaml.Unmarshal(data, &overrides)
		if err != nil {
			return errs.New("failed to parse override config YAML file: %v", err)
		}
		placementProducts = overrides.ProductPlacements
	case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
		// JSON string
		err := json.Unmarshal(strBytes, &placementProducts)
		if err != nil {
			return errs.New("failed to parse override config JSON: %v", err)
		}
	default:
		// YAML string
		err := yaml.Unmarshal(strBytes, &placementProducts)
		if err != nil {
			return errs.New("failed to parse override config YAML: %v", err)
		}
	}

	placementProductMap := make(map[int]int32)
	for productID, placements := range placementProducts {
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

	data, err := yaml.Marshal(mapping)
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

	s = strings.TrimSpace(s)
	strBytes := []byte(s)

	var partnerProductPlacements map[string]map[int32][]int
	switch {
	case strings.HasSuffix(s, ".yaml"):
		// YAML file path
		data, err := os.ReadFile(s)
		if err != nil {
			return errs.New("Couldn't read product config file from %s: %v", s, err)
		}

		var overrides PlacementOverrides
		err = yaml.Unmarshal(data, &overrides)
		if err != nil {
			return errs.New("failed to parse override config YAML file: %v", err)
		}
		partnerProductPlacements = overrides.PartnerProductPlacements
	case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
		// JSON string
		err := json.Unmarshal(strBytes, &partnerProductPlacements)
		if err != nil {
			return errs.New("failed to parse override config JSON: %v", err)
		}
	default:
		// YAML string
		err := yaml.Unmarshal(strBytes, &partnerProductPlacements)
		if err != nil {
			return errs.New("failed to parse override config YAML: %v", err)
		}
	}

	partnerPlacementProductMap := make(map[string]PlacementProductMap)
	for partner, productPlacement := range partnerProductPlacements {
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
