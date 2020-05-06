// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package heldamount

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
)

// DB exposes all needed functionality to manage heldAmount.
//
// architecture: Service
type DB interface {
	// GetPaystub return payStub by nodeID and period.
	GetPaystub(ctx context.Context, nodeID storj.NodeID, period string) (PayStub, error)
	// GetAllPaystubs return all payStubs by nodeID.
	GetAllPaystubs(ctx context.Context, nodeID storj.NodeID) ([]PayStub, error)
	// CreatePaystub insert paystub into db.
	CreatePaystub(ctx context.Context, stub PayStub) (err error)
}

// ErrNoDataForPeriod represents errors from the heldamount database.
var ErrNoDataForPeriod = errs.Class("no payStub/payments for period error")

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

// Service is used to store and handle node paystub information
//
// architecture: Service
type Service struct {
	log *zap.Logger
	db  DB
}

// NewService returns a new Service
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
		return PayStub{}, err
	}

	return payStub, nil
}

// GetAllPaystubs returns all paystubs by nodeID.
func (service *Service) GetAllPaystubs(ctx context.Context, nodeID storj.NodeID) ([]PayStub, error) {
	payStubs, err := service.db.GetAllPaystubs(ctx, nodeID)
	if err != nil {
		return []PayStub{}, err
	}

	return payStubs, nil
}
