// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

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

func TestPlacementPriceOverrides(t *testing.T) {
	tests := []struct {
		id        string
		config    string
		expectErr bool
	}{
		// N.B. to match PlacementProductMap.String(), the placements and placements:products defined in tt.config must be sorted in increasing order.
		// PlacementProductMap.String() sorts the elements for a consistent result.
		{
			id:     "empty string",
			config: "",
		},
		{
			id:     "one placement, one product",
			config: "0:product0",
		},
		{
			id:     "multiple placements, one product",
			config: "0,1,2:product0",
		},
		{
			id:     "multiple placements, multiple products",
			config: "0,1,2:product0;3:product1",
		},
		{
			id:     "trailing semi-colon",
			config: "0,1:product0;",
		},
		{
			id:     "trailing double semi-colon",
			config: "0,1:product0;;",
		},
		{
			id:     "values separated by double semi-colon",
			config: "0,1:product0;;2,3:product1",
		},
		{
			id:        "product duplicated across multiple key-value pairs",
			config:    "0,1:product0;2,3:product0",
			expectErr: true,
		},
		{
			id:        "invalid config: no placements",
			config:    ":product0",
			expectErr: true,
		},
		{
			id:        "invalid config: no placements with comma",
			config:    ",:product0",
			expectErr: true,
		},
		{
			id:        "invalid config: placement assigned to multiple products",
			config:    "0:product0;0,1:product1",
			expectErr: true,
		},
		{
			id:        "invalid config: no colon",
			config:    "product0",
			expectErr: true,
		},
		{
			id:        "invalid config: double colon",
			config:    "0,1::product0",
			expectErr: true,
		},
		{
			id:        "invalid config: multiple colons",
			config:    "0,1:product0:product1",
			expectErr: true,
		},
		{
			id:        "invalid config: double comma",
			config:    "0,,1:product0",
			expectErr: true,
		},
		{
			id:        "invalid config: single placement not int",
			config:    "a:product0",
			expectErr: true,
		},
		{
			id:        "invalid config: multiple placements, one not int",
			config:    "0,a:product0",
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

			// *PlacementProductMap.Set ignores ';;' and trims trailing ';'.
			// Match this behavior to verify config string.
			config := strings.ReplaceAll(tt.config, ";;", ";")
			config = strings.TrimSuffix(config, ";")
			require.Equal(t, config, mapFromCfg.String())
		})
	}
}
