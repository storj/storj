// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/sync2"
)

// ErrChore is stripecoinpayments clearing loop chore error class.
var ErrChore = errs.Class("stripecoinpayments chore error")

// Chore runs clearing process of reconciling transactions deposits,
// customer balance, invoices and usages.
//
// architecture: Chore
type Chore struct {
	log                 *zap.Logger
	service             *Service
	TransactionCycle    sync2.Cycle
	CouponUsageCycle    sync2.Cycle
	AccountBalanceCycle sync2.Cycle
}

// NewChore creates new clearing loop chore.
// TODO: uncomment new interval when coupons will be finished.
func NewChore(log *zap.Logger, service *Service, txInterval, accBalanceInterval /* couponUsageInterval */ time.Duration) *Chore {
	return &Chore{
		log:              log,
		service:          service,
		TransactionCycle: *sync2.NewCycle(txInterval),
		// TODO: uncomment when coupons will be finished.
		//CouponUsageCycle:    *sync2.NewCycle(couponUsageInterval),
		AccountBalanceCycle: *sync2.NewCycle(accBalanceInterval),
	}
}

// Run runs all clearing related cycles.
func (chore *Chore) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	var group errgroup.Group

	chore.TransactionCycle.Start(ctx, &group,
		func(ctx context.Context) error {
			chore.log.Info("running transactions update cycle")

			if err := chore.service.updateTransactionsLoop(ctx); err != nil {
				chore.log.Error("transaction update cycle failed", zap.Error(ErrChore.Wrap(err)))
			}

			return nil
		},
	)
	chore.AccountBalanceCycle.Start(ctx, &group,
		func(ctx context.Context) error {
			chore.log.Info("running account balance update cycle")

			if err := chore.service.updateAccountBalanceLoop(ctx); err != nil {
				chore.log.Error("account balance update cycle failed", zap.Error(ErrChore.Wrap(err)))
			}

			return nil
		},
	)
	// TODO: uncomment when coupons will be finished.
	//chore.CouponUsageCycle.Start(ctx, &group,
	//	func(ctx context.Context) error {
	//		chore.log.Info("running coupon usage cycle")
	//
	//		if err := chore.service.updateCouponUsageLoop(ctx); err != nil {
	//			chore.log.Error("coupon usage cycle failed", zap.Error(ErrChore.Wrap(err)))
	//		}
	//
	//		return nil
	//	},
	//)

	return ErrChore.Wrap(group.Wait())
}

// Close closes all underlying resources.
func (chore *Chore) Close() (err error) {
	defer mon.Task()(nil)(&err)

	chore.TransactionCycle.Close()
	chore.AccountBalanceCycle.Close()
	return nil
}
