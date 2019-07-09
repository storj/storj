// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"storj.io/storj/satellite/payments/stripepayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// stripePaymentsDB is an implementation of stripepayments.DB
type stripePaymentsDB struct {
	db *dbx.DB
}

// UserPayments is a getter for UserPayments repository
func (db *stripePaymentsDB) UserPayments() stripepayments.UserPayments {
	return &userpayments{db: db.db}
}

// ProjectPayments is a getter for ProjectPayments repository
func (db *stripePaymentsDB) ProjectPayments() stripepayments.ProjectPayments {
	return &projectpayments{db: db.db}
}

// ProjectInvoiceStamps is a getter for ProjectInvoiceStamps repository
func (db *stripePaymentsDB) ProjectInvoiceStamps() stripepayments.ProjectInvoiceStamps {
	return &projectInvoiceStamps{db: db.db}
}
