// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/storj"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/snopayouts"
)

// snopayoutsDB is payment data for specific storagenode for some specific period by working with satellite.
//
// architecture: Database
type snopayoutsDB struct {
	db *satelliteDB
}

// GetPaystub returns payStub by nodeID and period.
func (db *snopayoutsDB) GetPaystub(ctx context.Context, nodeID storj.NodeID, period string) (paystub snopayouts.Paystub, err error) {
	dbxPaystub, err := db.db.Get_StoragenodePaystub_By_NodeId_And_Period(ctx,
		dbx.StoragenodePaystub_NodeId(nodeID.Bytes()),
		dbx.StoragenodePaystub_Period(period))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return snopayouts.Paystub{}, snopayouts.ErrNoDataForPeriod.Wrap(err)
		}
		return snopayouts.Paystub{}, Error.Wrap(err)
	}
	return convertDBXPaystub(dbxPaystub)
}

// GetAllPaystubs return all payStubs by nodeID.
func (db *snopayoutsDB) GetAllPaystubs(ctx context.Context, nodeID storj.NodeID) (paystubs []snopayouts.Paystub, err error) {
	dbxPaystubs, err := db.db.All_StoragenodePaystub_By_NodeId(ctx,
		dbx.StoragenodePaystub_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	for _, dbxPaystub := range dbxPaystubs {
		payStub, err := convertDBXPaystub(dbxPaystub)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		paystubs = append(paystubs, payStub)
	}
	return paystubs, nil
}

func convertDBXPaystub(dbxPaystub *dbx.StoragenodePaystub) (snopayouts.Paystub, error) {
	nodeID, err := storj.NodeIDFromBytes(dbxPaystub.NodeId)
	if err != nil {
		return snopayouts.Paystub{}, Error.Wrap(err)
	}
	return snopayouts.Paystub{
		Period:         dbxPaystub.Period,
		NodeID:         nodeID,
		Created:        dbxPaystub.CreatedAt,
		Codes:          dbxPaystub.Codes,
		UsageAtRest:    dbxPaystub.UsageAtRest,
		UsageGet:       dbxPaystub.UsageGet,
		UsagePut:       dbxPaystub.UsagePut,
		UsageGetRepair: dbxPaystub.UsageGetRepair,
		UsagePutRepair: dbxPaystub.UsagePutRepair,
		UsageGetAudit:  dbxPaystub.UsageGetAudit,
		CompAtRest:     dbxPaystub.CompAtRest,
		CompGet:        dbxPaystub.CompGet,
		CompPut:        dbxPaystub.CompPut,
		CompGetRepair:  dbxPaystub.CompGetRepair,
		CompPutRepair:  dbxPaystub.CompPutRepair,
		CompGetAudit:   dbxPaystub.CompGetAudit,
		SurgePercent:   dbxPaystub.SurgePercent,
		Held:           dbxPaystub.Held,
		Owed:           dbxPaystub.Owed,
		Disposed:       dbxPaystub.Disposed,
		Paid:           dbxPaystub.Paid,
		Distributed:    dbxPaystub.Distributed,
	}, nil
}

// GetPayment returns payment by nodeID and period.
func (db *snopayoutsDB) GetPayment(ctx context.Context, nodeID storj.NodeID, period string) (payment snopayouts.Payment, err error) {
	// N.B. There can be multiple payments for a single node id and period, but the old query
	// here did not take that into account. Indeed, all above layers do not take it into account
	// from the service endpoints to the protobuf rpcs to the node client side. Instead of fixing
	// all of those things now, emulate the behavior with dbx as much as possible.

	dbxPayments, err := db.db.Limited_StoragenodePayment_By_NodeId_And_Period_OrderBy_Desc_Id(ctx,
		dbx.StoragenodePayment_NodeId(nodeID.Bytes()),
		dbx.StoragenodePayment_Period(period),
		1, 0)
	if err != nil {
		return snopayouts.Payment{}, Error.Wrap(err)
	}

	switch len(dbxPayments) {
	case 0:
		return snopayouts.Payment{}, snopayouts.ErrNoDataForPeriod.Wrap(sql.ErrNoRows)
	case 1:
		return convertDBXPayment(dbxPayments[0])
	default:
		return snopayouts.Payment{}, Error.New("impossible number of rows returned: %d", len(dbxPayments))
	}
}

// GetAllPayments return all payments by nodeID.
func (db *snopayoutsDB) GetAllPayments(ctx context.Context, nodeID storj.NodeID) (payments []snopayouts.Payment, err error) {
	dbxPayments, err := db.db.All_StoragenodePayment_By_NodeId(ctx,
		dbx.StoragenodePayment_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, dbxPayment := range dbxPayments {
		payment, err := convertDBXPayment(dbxPayment)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		payments = append(payments, payment)
	}

	return payments, nil
}

func convertDBXPayment(dbxPayment *dbx.StoragenodePayment) (snopayouts.Payment, error) {
	nodeID, err := storj.NodeIDFromBytes(dbxPayment.NodeId)
	if err != nil {
		return snopayouts.Payment{}, Error.Wrap(err)
	}
	return snopayouts.Payment{
		ID:      dbxPayment.Id,
		Created: dbxPayment.CreatedAt,
		NodeID:  nodeID,
		Period:  dbxPayment.Period,
		Amount:  dbxPayment.Amount,
		Receipt: derefStringOr(dbxPayment.Receipt, ""),
		Notes:   derefStringOr(dbxPayment.Notes, ""),
	}, nil
}

func derefStringOr(v *string, def string) string {
	if v != nil {
		return *v
	}
	return def
}

//
// test helpers
//

// TestCreatePaystub inserts storagenode_paystub into database. Only used for tests.
func (db *snopayoutsDB) TestCreatePaystub(ctx context.Context, stub snopayouts.Paystub) (err error) {
	return db.db.ReplaceNoReturn_StoragenodePaystub(ctx,
		dbx.StoragenodePaystub_Period(stub.Period),
		dbx.StoragenodePaystub_NodeId(stub.NodeID.Bytes()),
		dbx.StoragenodePaystub_Codes(stub.Codes),
		dbx.StoragenodePaystub_UsageAtRest(stub.UsageAtRest),
		dbx.StoragenodePaystub_UsageGet(stub.UsageGet),
		dbx.StoragenodePaystub_UsagePut(stub.UsagePut),
		dbx.StoragenodePaystub_UsageGetRepair(stub.UsageGetRepair),
		dbx.StoragenodePaystub_UsagePutRepair(stub.UsagePutRepair),
		dbx.StoragenodePaystub_UsageGetAudit(stub.UsageGetAudit),
		dbx.StoragenodePaystub_CompAtRest(stub.CompAtRest),
		dbx.StoragenodePaystub_CompGet(stub.CompGet),
		dbx.StoragenodePaystub_CompPut(stub.CompPut),
		dbx.StoragenodePaystub_CompGetRepair(stub.CompGetRepair),
		dbx.StoragenodePaystub_CompPutRepair(stub.CompPutRepair),
		dbx.StoragenodePaystub_CompGetAudit(stub.CompGetAudit),
		dbx.StoragenodePaystub_SurgePercent(stub.SurgePercent),
		dbx.StoragenodePaystub_Held(stub.Held),
		dbx.StoragenodePaystub_Owed(stub.Owed),
		dbx.StoragenodePaystub_Disposed(stub.Disposed),
		dbx.StoragenodePaystub_Paid(stub.Paid),
		dbx.StoragenodePaystub_Distributed(stub.Distributed),
	)
}

// TestCreatePayment inserts storagenode_payment into database. Only used for tests.
func (db *snopayoutsDB) TestCreatePayment(ctx context.Context, payment snopayouts.Payment) (err error) {
	return db.db.CreateNoReturn_StoragenodePayment(ctx,
		dbx.StoragenodePayment_NodeId(payment.NodeID.Bytes()),
		dbx.StoragenodePayment_Period(payment.Period),
		dbx.StoragenodePayment_Amount(payment.Amount),
		dbx.StoragenodePayment_Create_Fields{
			Receipt: dbx.StoragenodePayment_Receipt(payment.Receipt),
			Notes:   dbx.StoragenodePayment_Notes(payment.Notes),
		},
	)
}
