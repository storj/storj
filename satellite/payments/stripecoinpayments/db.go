// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

type DB interface {
	Customers() CustomersDB
	Transactions() TransactionsDB
	ProjectRecords() ProjectRecordsDB
}
