// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

// ensures that *stripeCoinPaymentsDB implements stripecoinpayments.DB.
var _ stripecoinpayments.DB = (*stripeCoinPaymentsDB)(nil)

// stripeCoinPaymentsDB is stripecoinpayments DB.
//
// architecture: Database
type stripeCoinPaymentsDB struct {
	db *satelliteDB
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

// CouponsDB is getter for coupons db.
func (db *stripeCoinPaymentsDB) Coupons() stripecoinpayments.CouponsDB {
	return &coupons{db: db.db}
}
