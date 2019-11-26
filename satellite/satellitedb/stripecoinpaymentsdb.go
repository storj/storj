// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/satellite/payments/stripecoinpayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that *stripeCoinPaymentsDB implements stripecoinpayments.DB.
var _ stripecoinpayments.DB = (*stripeCoinPaymentsDB)(nil)

// stripeCoinPaymentsDB is stripecoinpayments DB.
//
// architecture: Database
type stripeCoinPaymentsDB struct {
	db *dbx.DB
}

// Customers is getter for customers db.
func (db *stripeCoinPaymentsDB) Customers() stripecoinpayments.CustomersDB {
	return &customers{db: db.db}
}

// Transactions is getter for transactions db.
func (db *stripeCoinPaymentsDB) Transactions() stripecoinpayments.TransactionsDB {
	return &coinPaymentsTransactions{db: db.db}
}

// ProjectRecords is getter for invoice project records db.
func (db *stripeCoinPaymentsDB) ProjectRecords() stripecoinpayments.ProjectRecordsDB {
	return &invoiceProjectRecords{db: db.db}
}
