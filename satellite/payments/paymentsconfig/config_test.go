// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package paymentsconfig_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func TestProjectUsagePriceOverrides(t *testing.T) {
	type Prices map[string]stripecoinpayments.ProjectUsagePriceModel

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
			testID:      "missing prices",
			configValue: "partner",
		}, {
			testID:      "missing partner",
			configValue: ":1,2,3",
		}, {
			testID:      "too few prices",
			configValue: "partner:1",
		}, {
			testID:      "single price override",
			configValue: "partner:1,2,3",
			expectedModel: Prices{
				// Shift is to change the precision from TB dollars to MB cents
				"partner": stripecoinpayments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(1).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(2).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(3).Shift(2),
				},
			},
		}, {
			testID:      "too many prices",
			configValue: "partner:1,2,3,4",
		}, {
			testID:      "invalid decimal",
			configValue: "partner:0.0.1,2,3",
		}, {
			testID:      "multiple price overrides",
			configValue: "partner1:1,2,3;partner2:4,5,6",
			expectedModel: Prices{
				"partner1": stripecoinpayments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(1).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(2).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(3).Shift(2),
				},
				"partner2": stripecoinpayments.ProjectUsagePriceModel{
					StorageMBMonthCents: decimal.NewFromInt(4).Shift(-4),
					EgressMBCents:       decimal.NewFromInt(5).Shift(-4),
					SegmentMonthCents:   decimal.NewFromInt(6).Shift(2),
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
			}
		})
	}
}
