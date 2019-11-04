// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/satellite/payments/stripecoinpayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type stripeCoinPaymentsDB struct {
	db *dbx.DB
}

func (db *stripeCoinPaymentsDB) Customers() stripecoinpayments.CustomersDB {
	return &customers{db:db.db}
}

func (db *stripeCoinPaymentsDB) Transactions() stripecoinpayments.TransactionsDB {
	return &coinPaymentsTransactions{db:db.db}
}

func (db *stripeCoinPaymentsDB) ProjectRecords() stripecoinpayments.ProjectRecordsDB {
	return &invoiceProjectRecords{db:db.db}
}

