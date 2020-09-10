// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package snopayout

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
)

// DB exposes all needed functionality to manage payout.
//
// architecture: Service
type DB interface {
	// GetPaystub return payStub by nodeID and period.
	GetPaystub(ctx context.Context, nodeID storj.NodeID, period string) (PayStub, error)
	// GetAllPaystubs return all payStubs by nodeID.
	GetAllPaystubs(ctx context.Context, nodeID storj.NodeID) ([]PayStub, error)
	// CreatePaystub insert paystub into db.
	CreatePaystub(ctx context.Context, stub PayStub) (err error)
	// GetPayment return storagenode payment by nodeID and period.
	GetPayment(ctx context.Context, nodeID storj.NodeID, period string) (StoragenodePayment, error)
	// CreatePayment insert payment into db.
	CreatePayment(ctx context.Context, payment StoragenodePayment) (err error)
	// GetAllPayments return all payments by nodeID.
	GetAllPayments(ctx context.Context, nodeID storj.NodeID) ([]StoragenodePayment, error)
}

// ErrNoDataForPeriod represents errors from the payout database.
var ErrNoDataForPeriod = errs.Class("no payStub/payments for period error")

// Error is the default error class for payout package.
var Error = errs.Class("payout db error")

// PayStub is an entity that holds held amount of cash that will be paid to storagenode operator after some period.
type PayStub struct {
	Period         string       `json:"period"`
	NodeID         storj.NodeID `json:"nodeId"`
	Created        time.Time    `json:"created"`
	Codes          string       `json:"codes"`
	UsageAtRest    float64      `json:"usageAtRest"`
	UsageGet       int64        `json:"usageGet"`
	UsagePut       int64        `json:"usagePut"`
	UsageGetRepair int64        `json:"usageGetRepair"`
	UsagePutRepair int64        `json:"usagePutRepair"`
	UsageGetAudit  int64        `json:"usageGetAudit"`
	CompAtRest     int64        `json:"compAtRest"`
	CompGet        int64        `json:"compGet"`
	CompPut        int64        `json:"compPut"`
	CompGetRepair  int64        `json:"compGetRepair"`
	CompPutRepair  int64        `json:"compPutRepair"`
	CompGetAudit   int64        `json:"compGetAudit"`
	SurgePercent   int64        `json:"surgePercent"`
	Held           int64        `json:"held"`
	Owed           int64        `json:"owed"`
	Disposed       int64        `json:"disposed"`
	Paid           int64        `json:"paid"`
}

// StoragenodePayment is an entity that holds payment to storagenode operator parameters.
type StoragenodePayment struct {
	ID      int64        `json:"id"`
	Created time.Time    `json:"created"`
	NodeID  storj.NodeID `json:"nodeId"`
	Period  string       `json:"period"`
	Amount  int64        `json:"amount"`
	Receipt string       `json:"receipt"`
	Notes   string       `json:"notes"`
}

// Service is used to store and handle node paystub information
//
// architecture: Service
type Service struct {
	log *zap.Logger
	db  DB
}

// NewService returns a new Service.
func NewService(log *zap.Logger, db DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}

// GetPayStub returns PayStub by nodeID and period.
func (service *Service) GetPayStub(ctx context.Context, nodeID storj.NodeID, period string) (PayStub, error) {
	payStub, err := service.db.GetPaystub(ctx, nodeID, period)
	if err != nil {
		return PayStub{}, Error.Wrap(err)
	}

	return payStub, nil
}

// GetAllPaystubs returns all paystubs by nodeID.
func (service *Service) GetAllPaystubs(ctx context.Context, nodeID storj.NodeID) ([]PayStub, error) {
	payStubs, err := service.db.GetAllPaystubs(ctx, nodeID)
	if err != nil {
		return []PayStub{}, Error.Wrap(err)
	}

	return payStubs, nil
}

// GetPayment returns storagenode payment data by nodeID and period.
func (service *Service) GetPayment(ctx context.Context, nodeID storj.NodeID, period string) (StoragenodePayment, error) {
	payment, err := service.db.GetPayment(ctx, nodeID, period)
	if err != nil {
		return StoragenodePayment{}, Error.Wrap(err)
	}

	return payment, nil
}

// GetAllPayments returns all payments by nodeID.
func (service *Service) GetAllPayments(ctx context.Context, nodeID storj.NodeID) ([]StoragenodePayment, error) {
	payments, err := service.db.GetAllPayments(ctx, nodeID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return payments, nil
}
