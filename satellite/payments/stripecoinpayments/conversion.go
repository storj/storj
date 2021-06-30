// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"math"
	"math/big"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// convertToCents convert amount to cents with given rate.
func convertToCents(rate, amount *big.Float) int64 {
	f, _ := new(big.Float).Mul(amount, rate).Float64()
	return int64(math.Round(f * 100))
}

// convertFromCents convert amount in cents to big.Float with given rate.
func convertFromCents(rate *big.Float, amount int64) *big.Float {
	a := new(big.Float).SetInt64(amount)
	a = a.Quo(a, new(big.Float).SetInt64(100))
	return new(big.Float).Quo(a, rate)
}

// ErrConversion defines version service error.
var ErrConversion = errs.Class("conversion service")

// ConversionService updates conversion rates in a loop.
//
// architecture: Service
type ConversionService struct {
	log     *zap.Logger
	service *Service
	Cycle   sync2.Cycle
}

// NewConversionService creates new instance of ConversionService.
func NewConversionService(log *zap.Logger, service *Service, interval time.Duration) *ConversionService {
	return &ConversionService{
		log:     log,
		service: service,
		Cycle:   *sync2.NewCycle(interval),
	}
}

// Run runs loop which updates conversion rates for service.
func (conversion *ConversionService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return ErrConversion.Wrap(conversion.Cycle.Run(ctx,
		func(ctx context.Context) error {
			conversion.log.Debug("running conversion rates update cycle")

			if err := conversion.service.UpdateRates(ctx); err != nil {
				conversion.log.Error("conversion rates update cycle failed", zap.Error(ErrChore.Wrap(err)))
			}

			return nil
		},
	))
}

// Close closes underlying cycle.
func (conversion *ConversionService) Close() (err error) {
	defer mon.Task()(nil)(&err)

	conversion.Cycle.Close()
	return nil
}
