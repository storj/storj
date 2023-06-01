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

func TestProjectUsagePriceOverrides(t *testing.T) {
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
			configValue: "partner",
		}, {
			testID:      "missing partner",
			configValue: ":1,2,3,4",
		}, {
			testID:      "too few values",
			configValue: "partner:1",
		}, {
			testID:      "single price override",
			configValue: "partner:1,2,3,4",
			expectedModel: Prices{
				// Shift is to change the precision from TB dollars to MB cents
				"partner": payments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(1).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(2).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(3).Shift(2),
					EgressDiscountRatio: 4,
				},
			},
		}, {
			testID:      "too many values",
			configValue: "partner:1,2,3,4,5",
		}, {
			testID:      "invalid price",
			configValue: "partner:0.0.1,2,3,4",
		}, {
			testID:      "multiple price overrides",
			configValue: "partner1:1,2,3,4;partner2:5,6,7,8",
			expectedModel: Prices{
				"partner1": payments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(1).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(2).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(3).Shift(2),
					EgressDiscountRatio: 4,
				},
				"partner2": payments.ProjectUsagePriceModel{
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
			price := &paymentsconfig.ProjectUsagePriceOverrides{}
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
			for partner, price := range c.expectedModel {
				model := models[partner]
				require.Contains(t, models, partner)
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
