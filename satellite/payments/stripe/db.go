// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

// DB is stripecoinpayments DB interface.
//
// architecture: Database
type DB interface {
	// Customers is getter for customers db.
	Customers() CustomersDB
	// Transactions is getter for transactions db.
	Transactions() TransactionsDB
	// ProjectRecords is getter for invoice project records db.
	ProjectRecords() ProjectRecordsDB
}
