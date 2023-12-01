// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"storj.io/common/memory"
	"storj.io/storj/satellite/payments/billing"
)

var _ billing.Observer = (*UpgradeUserObserver)(nil)

// UpgradeUserObserver used to upgrade user if their balance is more than $10 after confirmed token transaction.
type UpgradeUserObserver struct {
	consoleDB             DB
	transactionsDB        billing.TransactionsDB
	usageLimitsConfig     UsageLimitsConfig
	userBalanceForUpgrade int64
	freezeService         *AccountFreezeService
}

// NewUpgradeUserObserver creates new observer instance.
func NewUpgradeUserObserver(consoleDB DB, transactionsDB billing.TransactionsDB, usageLimitsConfig UsageLimitsConfig, userBalanceForUpgrade int64, freezeService *AccountFreezeService) *UpgradeUserObserver {
	return &UpgradeUserObserver{
		consoleDB:             consoleDB,
		transactionsDB:        transactionsDB,
		usageLimitsConfig:     usageLimitsConfig,
		userBalanceForUpgrade: userBalanceForUpgrade,
		freezeService:         freezeService,
	}
}

// Process puts user into the paid tier and converts projects to upgraded limits.
func (o *UpgradeUserObserver) Process(ctx context.Context, transaction billing.Transaction) (err error) {
	defer mon.Task()(&ctx)(&err)

	freezes, err := o.freezeService.GetAll(ctx, transaction.UserID)
	if err != nil {
		return err
	}

	if freezes.LegalFreeze != nil || freezes.ViolationFreeze != nil {
		// user can not exit these freezes by paying with tokens
		return nil
	}

	user, err := o.consoleDB.Users().Get(ctx, transaction.UserID)
	if err != nil {
		return err
	}

	if user.PaidTier {
		return nil
	}

	balance, err := o.transactionsDB.GetBalance(ctx, user.ID)
	if err != nil {
		return err
	}

	// check if user's balance is less than needed amount for upgrade.
	if balance.BaseUnits() < o.userBalanceForUpgrade {
		return nil
	}

	err = o.consoleDB.Users().UpdatePaidTier(ctx, user.ID, true,
		o.usageLimitsConfig.Bandwidth.Paid,
		o.usageLimitsConfig.Storage.Paid,
		o.usageLimitsConfig.Segment.Paid,
		o.usageLimitsConfig.Project.Paid,
	)
	if err != nil {
		return err
	}

	projects, err := o.consoleDB.Projects().GetOwn(ctx, user.ID)
	if err != nil {
		return err
	}
	for _, project := range projects {
		if project.StorageLimit == nil || *project.StorageLimit < o.usageLimitsConfig.Storage.Paid {
			project.StorageLimit = new(memory.Size)
			*project.StorageLimit = o.usageLimitsConfig.Storage.Paid
		}
		if project.BandwidthLimit == nil || *project.BandwidthLimit < o.usageLimitsConfig.Bandwidth.Paid {
			project.BandwidthLimit = new(memory.Size)
			*project.BandwidthLimit = o.usageLimitsConfig.Bandwidth.Paid
		}
		if project.SegmentLimit == nil || *project.SegmentLimit < o.usageLimitsConfig.Segment.Paid {
			*project.SegmentLimit = o.usageLimitsConfig.Segment.Paid
		}
		err = o.consoleDB.Projects().Update(ctx, &project)
		if err != nil {
			return err
		}
	}

	return nil
}
