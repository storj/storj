// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
)

func TestPriceOverrides(t *testing.T) {
	type Prices map[string]payments.ProjectUsagePriceModel

	cases := []struct {
		testID        string
		configValue   string
		expectedModel Prices
	}{
		{
			testID:        "empty",
			configValue:   "",
			expectedModel: Prices{},
		}, {
			testID:      "missing values",
			configValue: "key0",
		}, {
			testID:      "missing key",
			configValue: ":1,2,3,4",
		}, {
			testID:      "too few values",
			configValue: "key0:1",
		}, {
			testID:      "single price override",
			configValue: "key0:1,2,3,4",
			expectedModel: Prices{
				// Shift is to change the precision from TB dollars to MB cents
				"key0": payments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(1).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(2).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(3).Shift(2),
					EgressDiscountRatio: 4,
				},
			},
		}, {
			testID:      "too many values",
			configValue: "key0:1,2,3,4,5",
		}, {
			testID:      "invalid price",
			configValue: "key0:0.0.1,2,3,4",
		}, {
			testID:      "multiple price overrides",
			configValue: "key1:1,2,3,4;key2:5,6,7,8",
			expectedModel: Prices{
				"key1": payments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(1).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(2).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(3).Shift(2),
					EgressDiscountRatio: 4,
				},
				"key2": payments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(5).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(6).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(7).Shift(2),
					EgressDiscountRatio: 8,
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.testID, func(t *testing.T) {
			price := &paymentsconfig.PriceOverrides{}
			err := price.Set(c.configValue)
			if c.expectedModel == nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			strParts := strings.Split(price.String(), ";")
			sort.Strings(strParts)
			require.Equal(t, c.configValue, strings.Join(strParts, ";"))

			models, err := price.ToModels()
			require.NoError(t, err)
			require.Len(t, models, len(c.expectedModel))
			for key, price := range c.expectedModel {
				model := models[key]
				require.Contains(t, models, key)
				require.Equal(t, price.StorageMBMonthCents, model.StorageMBMonthCents)
				require.Equal(t, price.EgressMBCents, model.EgressMBCents)
				require.Equal(t, price.SegmentMonthCents, model.SegmentMonthCents)
				require.Equal(t, price.EgressDiscountRatio, model.EgressDiscountRatio)
			}
		})
	}
}

func TestPackagePlans(t *testing.T) {
	type packages map[string]payments.PackagePlan

	cases := []struct {
		testID               string
		configValue          string
		expectedPackagePlans packages
	}{
		{
			testID:               "empty",
			configValue:          "",
			expectedPackagePlans: packages{},
		},
		{
			testID:      "missing couponID and price",
			configValue: "partner",
		},
		{
			testID:      "missing partner",
			configValue: ":100,100",
		}, {
			testID:      "empty price",
			configValue: "partner:,100",
		}, {
			testID:      "empty credit",
			configValue: "partner:100,",
		},
		{
			testID:      "too few values",
			configValue: "partner:100",
		},
		{
			testID:      "too many values",
			configValue: "partner:100,100,200",
		},
		{
			testID:      "single package plan",
			configValue: "partner1:100,200",
			expectedPackagePlans: packages{
				"partner1": payments.PackagePlan{
					Price:  100,
					Credit: 200,
				},
			},
		},
		{
			testID:      "multiple package plans",
			configValue: "partner1:100,200;partner2:200,300",
			expectedPackagePlans: packages{
				"partner1": payments.PackagePlan{
					Price:  100,
					Credit: 200,
				},
				"partner2": payments.PackagePlan{
					Price:  200,
					Credit: 300,
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.testID, func(t *testing.T) {
			packagePlans := paymentsconfig.PackagePlans{}
			err := packagePlans.Set(c.configValue)
			if c.expectedPackagePlans == nil {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			strParts := strings.Split(packagePlans.String(), ";")
			sort.Strings(strParts)
			require.Equal(t, c.configValue, strings.Join(strParts, ";"))

			for k, v := range c.expectedPackagePlans {
				p, err := packagePlans.Get([]byte(k))
				require.NoError(t, err)
				require.Equal(t, v, p)
			}
		})
	}
}

func TestPackagePlansGet(t *testing.T) {
	partner := "partnerName1"
	credit := int64(200)
	price := int64(100)
	configStr := fmt.Sprintf("%s:%d,%d", partner, price, credit)

	packagePlans := paymentsconfig.PackagePlans{}
	require.NoError(t, packagePlans.Set(configStr))

	cases := []struct {
		testID     string
		userAgent  []byte
		shouldPass bool
	}{
		{
			testID:     "user agent matches partner",
			userAgent:  []byte(partner),
			shouldPass: true,
		},
		{
			testID:     "partner is first entry of user agent",
			userAgent:  []byte(partner + "/0.1.2"),
			shouldPass: true,
		},
		{
			testID:     "partner is not first entry of user agent",
			userAgent:  []byte("app2/1.2.3 " + partner + "/1.2.3"),
			shouldPass: true,
		},
		{
			testID:     "partner is a prefix of user agent, but not equal",
			userAgent:  []byte("partnerName12/1.2.3"),
			shouldPass: false,
		},
		{
			testID:     "partner does not exist in user agent",
			userAgent:  []byte("partnerName2/1.2.3"),
			shouldPass: false,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			p, err := packagePlans.Get(c.userAgent)
			if c.shouldPass {
				require.NoError(t, err)
				require.Equal(t, credit, p.Credit)
				require.Equal(t, price, p.Price)
			} else {
				require.Error(t, err)
				require.Empty(t, p)
			}

		})
	}
}

var placementOverrides = paymentsconfig.PlacementOverrides{
	ProductPlacements: map[int32][]int{
		1: {0},
	},
}

func TestPlacementPriceOverrides(t *testing.T) {
	bytes, err := yaml.Marshal(placementOverrides.ProductPlacements)
	require.NoError(t, err)
	validYaml := string(bytes)

	bytes, err = json.Marshal(placementOverrides.ProductPlacements)
	require.NoError(t, err)
	jsonStr := string(bytes)

	tmpFile, err := os.CreateTemp(t.TempDir(), "mapping_*.yaml")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove(tmpFile.Name()))
		require.NoError(t, tmpFile.Close())
	}()

	bytes, err = yaml.Marshal(placementOverrides)
	require.NoError(t, err)
	_, err = tmpFile.Write(bytes)
	require.NoError(t, err)

	tests := []struct {
		id     string
		config string
		// in the case of JSON, we only allow using it for backwards compatibility
		// the expected config string of cfg.String() will be in YAML format.
		expectStr string
		expectErr bool
	}{
		{
			id:     "empty string",
			config: "",
		},
		{
			id:     "valid YAML",
			config: validYaml,
		},
		{
			id:        "YAML file",
			config:    tmpFile.Name(),
			expectStr: validYaml,
		},
		{
			id:        "valid JSON",
			config:    jsonStr,
			expectStr: validYaml,
		},
		{
			id:        "invalid string",
			config:    "invalid string",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			mapFromCfg := &paymentsconfig.PlacementProductMap{}
			err := mapFromCfg.Set(tt.config)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.expectStr != "" {
				require.Equal(t, tt.expectStr, mapFromCfg.String())
				return
			}
			require.Equal(t, tt.config, mapFromCfg.String())
		})
	}
}

func TestProductPriceOverrides(t *testing.T) {
	const seatsOnlyName = "Object Mount (Any Cloud)"

	product := func(p paymentsconfig.ProductUsagePriceYaml) string {
		bytes, err := yaml.Marshal([]paymentsconfig.ProductUsagePriceYaml{p})
		require.NoError(t, err)
		return string(bytes)
	}
	withFields := func(base paymentsconfig.ProductUsagePriceYaml, mutate func(*paymentsconfig.ProductUsagePriceYaml)) string {
		mutate(&base)
		return product(base)
	}

	validYaml := product(paymentsconfig.ProductUsagePriceYaml{
		ID:                  1,
		Name:                "select-product",
		Storage:             "5",
		StorageSKU:          "storage",
		Egress:              "6",
		EgressSKU:           "egress",
		Segment:             "6",
		SegmentSKU:          "segment",
		EgressDiscountRatio: "0.10",
		LicenseFee:          "29.00",
		LicenseFeeSKU:       "license",
	})

	tmpFile, err := os.CreateTemp(t.TempDir(), "products_*.yaml")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove(tmpFile.Name()))
		require.NoError(t, tmpFile.Close())
	}()

	_, err = tmpFile.WriteString(validYaml)
	require.NoError(t, err)

	// seatsOnly sells license seats and nothing else, so it has no usage to price.
	seatsOnly := paymentsconfig.ProductUsagePriceYaml{
		ID:                  8,
		Name:                seatsOnlyName,
		LicenseFee:          "39.00",
		LicenseFeeSKU:       "OM-ANYCLOUD-SEAT",
		EgressDiscountRatio: "0.00",
	}
	// usageOnly bills usage and no seats.
	usageOnly := paymentsconfig.ProductUsagePriceYaml{
		ID:                  8,
		Name:                seatsOnlyName,
		Storage:             "4",
		Egress:              "7",
		EgressDiscountRatio: "0.00",
	}

	tests := []struct {
		id     string
		config string
		// in the case of JSON, we only allow using it for backwards compatibility
		// the expected config string of cfg.String() will be in YAML format.
		expectStr string
		expectErr bool

		// ToModel() expectations.
		errContains []string
		productID   int32
		feeCents    string
		feeSKU      string
		zeroUsage   bool
	}{
		{
			id:     "empty string",
			config: "",
		},
		{
			id:        "valid YAML",
			config:    validYaml,
			productID: 1,
			feeCents:  "2900",
			feeSKU:    "license",
		},
		{
			id:        "YAML file",
			config:    tmpFile.Name(),
			expectStr: validYaml,
			productID: 1,
			feeCents:  "2900",
			feeSKU:    "license",
		},
		{
			id:        "invalid YAML",
			config:    "invalid string",
			expectErr: true,
		},
		{
			id:        "absent license fee is zero",
			config:    product(usageOnly),
			productID: 8,
			feeCents:  "0",
		},
		{
			id: "malformed license fee is rejected",
			config: withFields(usageOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.LicenseFee = "not-a-number"
			}),
			errContains: []string{"can't convert"},
		},
		{
			id:        "a seats-only product may omit its usage prices",
			config:    product(seatsOnly),
			productID: 8,
			feeCents:  "3900",
			feeSKU:    "OM-ANYCLOUD-SEAT",
			zeroUsage: true,
		},
		{
			id: "stating zero usage prices is equivalent",
			config: withFields(seatsOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.Storage, p.Egress = "0", "0"
			}),
			productID: 8,
			feeCents:  "3900",
			feeSKU:    "OM-ANYCLOUD-SEAT",
			zeroUsage: true,
		},
		{
			// Pricing one side means the product bills usage, so the missing
			// counterpart is a mistake rather than a seats-only product.
			id: "a seats product pricing only storage is rejected",
			config: withFields(seatsOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.Storage = "4"
			}),
			errContains: []string{"product 8", "egress price is required"},
		},
		{
			id: "a seats product pricing only egress is rejected",
			config: withFields(seatsOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.Egress = "7"
			}),
			errContains: []string{"product 8", "storage price is required"},
		},
		{
			// A fee of zero sells no seats, so it is not a seats-only product.
			id: "a zero license fee does not excuse omitted usage prices",
			config: withFields(seatsOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.LicenseFee = "0"
			}),
			errContains: []string{"storage price is required"},
		},
		{
			id: "a product with no license fee must state storage",
			config: withFields(usageOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.Storage = ""
			}),
			errContains: []string{"product 8", "storage price is required"},
		},
		{
			id: "a product with no license fee must state egress",
			config: withFields(usageOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.Egress = ""
			}),
			errContains: []string{"product 8", "egress price is required"},
		},
		{
			// The decimal error names neither the product nor the field.
			id: "a malformed usage price names the product",
			config: withFields(usageOnly, func(p *paymentsconfig.ProductUsagePriceYaml) {
				p.Storage = "not-a-number"
			}),
			errContains: []string{"product 8", seatsOnlyName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			mapFromCfg := &paymentsconfig.ProductPriceOverrides{}
			err := mapFromCfg.Set(tt.config)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.expectStr != "" {
				require.Equal(t, tt.expectStr, mapFromCfg.String())
			} else {
				require.Equal(t, tt.config, mapFromCfg.String())
			}

			models, err := mapFromCfg.ToModels()
			if len(tt.errContains) > 0 {
				require.Error(t, err)
				for _, want := range tt.errContains {
					require.Contains(t, err.Error(), want)
				}
				return
			}
			require.NoError(t, err)
			if tt.productID == 0 {
				return
			}

			require.Contains(t, models, tt.productID)
			model := models[tt.productID]
			require.Equal(t, tt.feeCents, model.LicenseFeeCents.String())
			require.Equal(t, tt.feeSKU, model.LicenseFeeSKU)

			if tt.zeroUsage {
				require.True(t, model.StorageMBMonthCents.IsZero())
				require.True(t, model.EgressMBCents.IsZero())
				require.True(t, model.SegmentMonthCents.IsZero())
			} else {
				require.False(t, model.StorageMBMonthCents.IsZero())
				require.False(t, model.EgressMBCents.IsZero())
			}
		})
	}
}
