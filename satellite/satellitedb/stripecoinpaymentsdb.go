// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/satellite/payments/stripe"
)

// ensures that *stripeCoinPaymentsDB implements stripecoinpayments.DB.
var _ stripe.DB = (*stripeCoinPaymentsDB)(nil)

// stripeCoinPaymentsDB is stripecoinpayments DB.
//
// architecture: Database
type stripeCoinPaymentsDB struct {
	db *satelliteDB
}

// Customers is getter for customers db.
func (db *stripeCoinPaymentsDB) Customers() stripe.CustomersDB {
	return &customers{db: db.db}
}

// Transactions is getter for transactions db.
func (db *stripeCoinPaymentsDB) Transactions() stripe.TransactionsDB {
	return &coinPaymentsTransactions{db: db.db}
}

// ProjectRecords is getter for invoice project records db.
func (db *stripeCoinPaymentsDB) ProjectRecords() stripe.ProjectRecordsDB {
	return &invoiceProjectRecords{db: db.db}
}
