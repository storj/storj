// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package monetary

import (
	"math/big"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// If we use more than 18 decimal places, baseUnits values will overflow int64
	// even if there is only one digit to the left of the decimal point.
	manyDigitsCurrency = &Currency{name: "manyDigitsCurrency", symbol: "mdc", decimalPlaces: 18}
	noDigitsCurrency   = &Currency{name: "noDigitsCurrency", symbol: "ndc", decimalPlaces: 0}
)

func TestAmountFromBigFloatAndAmountAsBigFloat(t *testing.T) {
	parseFloat := func(s string) *big.Float {
		bf, _, err := big.ParseFloat(s, 10, 256, big.ToNearestEven)
		if err != nil {
			t.Fatalf("failed to parse %q as float: %v", s, err)
		}
		return bf
	}

	tests := []struct {
		name       string
		floatValue *big.Float
		currency   *Currency
		baseUnits  int64
		wantErr    bool
	}{
		{"zero", big.NewFloat(0), StorjToken, 0, false},
		{"one", big.NewFloat(1), USDollars, 100, false},
		{"negative", big.NewFloat(-1), Bitcoin, -100000000, false},
		{"smallest", big.NewFloat(1e-8), StorjToken, 1, false},
		{"minus smallest", big.NewFloat(-1e-8), StorjToken, -1, false},
		{"one+delta", parseFloat("1.000000000000000001"), manyDigitsCurrency, 1000000000000000001, false},
		{"minus one+delta", parseFloat("-1.000000000000000001"), manyDigitsCurrency, -1000000000000000001, false},
		{"large number", parseFloat("4611686018427387904.0"), noDigitsCurrency, 4611686018427387904, false},
		{"infinity", parseFloat("Inf"), StorjToken, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AmountFromBigFloat(tt.floatValue, tt.currency)
			if (err != nil) != tt.wantErr {
				t.Errorf("AmountFromBigFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			want := Amount{baseUnits: tt.baseUnits, currency: tt.currency}
			assert.Equal(t, want, got)
			assert.Equal(t, tt.baseUnits, got.BaseUnits())

			fullPrecValue, err := tt.floatValue.MarshalText()
			require.NoError(t, err)
			gotAsFloat := got.AsBigFloatWithPrecision(tt.floatValue.Prec())
			fullPrecGot, err := gotAsFloat.MarshalText()
			require.NoError(t, err)

			assert.Truef(t, tt.floatValue.Cmp(gotAsFloat) == 0,
				"(expected) %v != (got) %v", string(fullPrecValue), string(fullPrecGot))
		})
	}
}
func TestAmountFromDecimalAndAmountAsDecimal(t *testing.T) {
	tests := []struct {
		name         string
		decimalValue decimal.Decimal
		currency     *Currency
		baseUnits    int64
		wantErr      bool
	}{
		{"zero", decimal.Decimal{}, StorjToken, 0, false},
		{"one", decimal.NewFromInt(1), USDollars, 100, false},
		{"negative", decimal.NewFromInt(-1), Bitcoin, -100000000, false},
		{"smallest", decimal.NewFromFloat(1e-8), StorjToken, 1, false},
		{"one+delta", decimal.RequireFromString("1.000000000000000001"), manyDigitsCurrency, 1000000000000000001, false},
		{"minus one+delta", decimal.RequireFromString("-1.000000000000000001"), manyDigitsCurrency, -1000000000000000001, false},
		{"large number", decimal.RequireFromString("4611686018427387904.0"), noDigitsCurrency, 4611686018427387904, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AmountFromDecimal(tt.decimalValue, tt.currency)
			want := Amount{baseUnits: tt.baseUnits, currency: tt.currency}
			assert.Equal(t, want, got)
			assert.Equal(t, tt.baseUnits, got.BaseUnits())
			assert.Truef(t, tt.decimalValue.Equal(got.AsDecimal()),
				"%v != %v", tt.decimalValue, got.AsDecimal())
		})
	}
}
