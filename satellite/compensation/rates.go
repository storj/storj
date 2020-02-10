package compensation

import (
	"github.com/shopspring/decimal"
	"github.com/spf13/pflag"
)

type Rates struct {
	AtRestGBHours Rate
	GetTB         Rate
	PutTB         Rate
	GetRepairTB   Rate
	PutRepairTB   Rate
	GetAuditTB    Rate
}

type Rate decimal.Decimal

var _ pflag.Value = (*Rate)(nil)

func RateFromString(value string) (Rate, error) {
	r, err := decimal.NewFromString(value)
	if err != nil {
		return Rate{}, err
	}
	return Rate(r), nil
}

func (rate Rate) String() string {
	return decimal.Decimal(rate).String()
}

func (rate *Rate) Set(s string) error {
	r, err := decimal.NewFromString(s)
	if err != nil {
		return err
	}
	*rate = Rate(r)
	return nil
}

func (rate Rate) Type() string {
	return "rate"
}

func RequireRateFromString(s string) Rate {
	return Rate(decimal.RequireFromString(s))
}
