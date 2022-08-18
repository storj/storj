// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package monetary

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"
)

// Currency represents a currency for the purpose of representing amounts in
// that currency. Currency instances have a name, a symbol, and a number of
// supported decimal places of supported precision.
type Currency struct {
	name          string
	symbol        string
	decimalPlaces int32
}

// NewCurrency creates a new Currency instance.
func NewCurrency(name, symbol string, decimalPlaces int32) *Currency {
	return &Currency{name: name, symbol: symbol, decimalPlaces: decimalPlaces}
}

// Name returns the name of the currency.
func (c *Currency) Name() string {
	return c.name
}

// Symbol returns the symbol of the currency.
func (c *Currency) Symbol() string {
	return c.symbol
}

var (
	// StorjToken is the currency for the STORJ ERC20 token, which powers
	// most payments on the current Storj network.
	StorjToken = NewCurrency("STORJ Token", "STORJ", 8)
	// USDollars is the currency of United States dollars, where fractional
	// cents are not supported.
	USDollars = NewCurrency("US dollars", "USD", 2)
	// USDollarsMicro is the currency of United States dollars, where fractional
	// cents are supported with 2 decimal places.
	USDollarsMicro = NewCurrency("US dollars", "USDMicro", 6)
	// Bitcoin is the currency for the well-known cryptocurrency Bitcoin
	// (a.k.a. BTC).
	Bitcoin = NewCurrency("Bitcoin (BTC)", "BTC", 8)
	// LiveGoats is the currency of live goats, which some Storj network
	// satellites may elect to support for payments.
	LiveGoats = NewCurrency("Live goats", "goats", 0)

	// Error is a class of errors encountered in the monetary package.
	Error = errs.Class("monetary error")
)

// CurrencyFromSymbol returns currency based on symbol.
func CurrencyFromSymbol(symbol string) (*Currency, error) {
	switch symbol {
	case "STORJ":
		return StorjToken, nil
	case "BTC":
		return Bitcoin, nil
	case "USD":
		return USDollars, nil
	case "USDMicro":
		return USDollarsMicro, nil
	case "goats":
		return LiveGoats, nil
	default:
		return nil, errs.New("invalid currency symbol")
	}
}

// Amount represents a monetary amount, encapsulating a value and a currency.
//
// The value of the Amount is represented in "base units", meaning units of the
// smallest indivisible portion of the given currency. For example, when
// the currency is USDollars, the base unit would be cents.
type Amount struct {
	baseUnits int64
	currency  *Currency
}

// AsFloat returns the monetary value in currency units expressed as a
// floating point number. _Warning_ may lose precision! (float64 has the
// equivalent of 53 bits of precision, as defined by big.Float.)
func (a Amount) AsFloat() float64 {
	return float64(a.baseUnits) * math.Pow10(int(-a.currency.decimalPlaces))
}

// AsBigFloat returns the monetary value in currency units expressed as an
// instance of *big.Float with precision=64. _Warning_ may lose precision!
func (a Amount) AsBigFloat() *big.Float {
	return a.AsBigFloatWithPrecision(64)
}

// AsBigFloatWithPrecision returns the monetary value in currency units
// expressed as an instance of *big.Float with the given precision.
// _Warning_ this may lose precision if the specified precision is not
// large enough!
func (a Amount) AsBigFloatWithPrecision(p uint) *big.Float {
	stringVal := a.AsDecimal().String()
	bf, _, err := big.ParseFloat(stringVal, 10, p, big.ToNearestEven)
	if err != nil {
		// it does not appear that this is possible, after a review of
		// decimal.Decimal{}.String() and big.ParseFloat().
		panic(fmt.Sprintf("could not parse output of Decimal.String() (%q) as big.Float: %v", stringVal, err))
	}
	return bf
}

// AsDecimal returns the monetary value in currency units expressed as an
// arbitrary precision decimal number.
func (a Amount) AsDecimal() decimal.Decimal {
	d := decimal.NewFromInt(a.baseUnits)
	return d.Shift(-a.currency.decimalPlaces)
}

// BaseUnits returns the monetary value expressed in its base units.
func (a Amount) BaseUnits() int64 {
	return a.baseUnits
}

// Currency returns the currency of the amount.
func (a Amount) Currency() *Currency {
	return a.currency
}

// Equal returns true if a and other are in the same currency and have the
// same value.
func (a Amount) Equal(other Amount) bool {
	return a.currency == other.currency && a.baseUnits == other.baseUnits
}

// amountJSON is amount json data structure.
type amountJSON struct {
	Value    decimal.Decimal `json:"value"`
	Currency string          `json:"currency"`
}

// UnmarshalJSON unmarshals json bytes into amount.
func (a *Amount) UnmarshalJSON(data []byte) error {
	var amountJSON amountJSON
	if err := json.Unmarshal(data, &amountJSON); err != nil {
		return err
	}

	curr, err := CurrencyFromSymbol(amountJSON.Currency)
	if err != nil {
		return err
	}

	*a = AmountFromDecimal(amountJSON.Value, curr)
	return nil
}

// MarshalJSON marshals amount into json.
func (a Amount) MarshalJSON() ([]byte, error) {
	amountJSON := amountJSON{
		Value:    a.AsDecimal(),
		Currency: a.currency.symbol,
	}

	return json.Marshal(amountJSON)
}

// AmountFromBaseUnits creates a new Amount instance from the given count of
// base units and in the given currency.
func AmountFromBaseUnits(units int64, currency *Currency) Amount {
	return Amount{
		baseUnits: units,
		currency:  currency,
	}
}

// AmountFromDecimal creates a new Amount instance from the given decimal
// value and in the given currency. The decimal value is expected to be in
// currency units.
//
// Example:
//
//	AmountFromDecimal(decimal.NewFromFloat(3.50), USDollars) == Amount{baseUnits: 350, currency: USDollars}
func AmountFromDecimal(d decimal.Decimal, currency *Currency) Amount {
	return AmountFromBaseUnits(d.Shift(currency.decimalPlaces).Round(0).IntPart(), currency)
}

// AmountFromString creates a new Amount instance from the given base 10
// value and in the given currency. The string is expected to give the
// value of the amount in currency units.
func AmountFromString(s string, currency *Currency) (Amount, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Amount{}, Error.Wrap(err)
	}
	return AmountFromDecimal(d, currency), nil
}

// AmountFromBigFloat creates a new Amount instance from the given floating
// point value and in the given currency. The big.Float is expected to give
// the value of the amount in currency units.
func AmountFromBigFloat(f *big.Float, currency *Currency) (Amount, error) {
	dec, err := DecimalFromBigFloat(f)
	if err != nil {
		return Amount{}, err
	}
	return AmountFromDecimal(dec, currency), nil
}

// DecimalFromBigFloat creates a new decimal.Decimal instance from the given
// floating point value.
func DecimalFromBigFloat(f *big.Float) (decimal.Decimal, error) {
	if f.IsInf() {
		return decimal.Decimal{}, Error.New("Cannot represent infinite amount")
	}
	// This is probably not computationally ideal, but it should be the most
	// straightforward way to convert (unless/until the decimal package adds
	// a NewFromBigFloat method).
	stringVal := f.Text('e', -1)
	dec, err := decimal.NewFromString(stringVal)
	return dec, Error.Wrap(err)
}
