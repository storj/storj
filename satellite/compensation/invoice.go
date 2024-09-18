// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"

	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

// Invoice holds the calculations for the amount required to pay to a node
// for a given pay period.
type Invoice struct {
	Period             Period             `csv:"period"`               // The payment period
	NodeID             NodeID             `csv:"node-id"`              // The node ID
	NodeCreatedAt      UTCDate            `csv:"node-created-at"`      // When the node was created
	NodeDisqualified   *UTCDate           `csv:"node-disqualified"`    // When and if the node was disqualified
	NodeGracefulExit   *UTCDate           `csv:"node-gracefulexit"`    // When and if the node finished a graceful exit
	NodeWallet         string             `csv:"node-wallet"`          // The node's wallet address
	NodeWalletFeatures WalletFeatures     `csv:"node-wallet-features"` // The node's wallet features
	NodeAddress        string             `csv:"node-address"`         // The node's TODO
	NodeLastIP         string             `csv:"node-last-ip"`         // The last known ip the node had
	Codes              Codes              `csv:"codes"`                // Any codes providing context to the invoice
	UsageAtRest        float64            `csv:"usage-at-rest"`        // Byte-hours provided during the payment period
	UsageGet           int64              `csv:"usage-get"`            // Number of bytes served in GET requests
	UsagePut           int64              `csv:"usage-put"`            // Number of bytes served in PUT requests
	UsageGetRepair     int64              `csv:"usage-get-repair"`     // Number of bytes served in GET_REPAIR requests
	UsagePutRepair     int64              `csv:"usage-put-repair"`     // Number of bytes served in PUT_REPAIR requests
	UsageGetAudit      int64              `csv:"usage-get-audit"`      // Number of bytes served in GET_AUDIT requests
	CompAtRest         currency.MicroUnit `csv:"comp-at-rest"`         // Compensation for usage-at-rest
	CompGet            currency.MicroUnit `csv:"comp-get"`             // Compensation for usage-get
	CompPut            currency.MicroUnit `csv:"comp-put"`             // Compensation for usage-put
	CompGetRepair      currency.MicroUnit `csv:"comp-get-repair"`      // Compensation for usage-get-repair
	CompPutRepair      currency.MicroUnit `csv:"comp-put-repair"`      // Compensation for usage-put-repair
	CompGetAudit       currency.MicroUnit `csv:"comp-get-audit"`       // Compensation for usage-get-audit
	SurgePercent       int64              `csv:"surge-percent"`        // Surge percent used to calculate compensation, or 0 if no surge
	Owed               currency.MicroUnit `csv:"owed"`                 // Amount we intend to pay to the node (sum(comp-*) - held + disposed)
	Held               currency.MicroUnit `csv:"held"`                 // Amount held from sum(comp-*) for this period
	Disposed           currency.MicroUnit `csv:"disposed"`             // Amount of owed that is due to graceful-exit or held period ending
	TotalHeld          currency.MicroUnit `csv:"total-held"`           // Total amount ever held from the node
	TotalDisposed      currency.MicroUnit `csv:"total-disposed"`       // Total amount ever disposed to the node
	TotalPaid          currency.MicroUnit `csv:"total-paid"`           // Total amount ever paid to the node (but not necessarily dispensed)
	TotalDistributed   currency.MicroUnit `csv:"total-distributed"`    // Total amount ever distributed to the node (always less than or equal to paid)
}

// MergeNodeInfo updates the fields representing the node information into the invoice.
func (invoice *Invoice) MergeNodeInfo(nodeInfo NodeInfo) error {
	if invoice.NodeID != NodeID(nodeInfo.ID) {
		return Error.New("node ID mismatch (invoice=%q nodeinfo=%q)", invoice.NodeID, nodeInfo.ID)
	}
	invoice.NodeCreatedAt = UTCDate(nodeInfo.CreatedAt)
	invoice.NodeDisqualified = (*UTCDate)(nodeInfo.Disqualified)
	invoice.NodeGracefulExit = (*UTCDate)(nodeInfo.GracefulExit)
	invoice.UsageAtRest = nodeInfo.UsageAtRest
	invoice.UsageGet = nodeInfo.UsageGet
	invoice.UsagePut = nodeInfo.UsagePut
	invoice.UsageGetRepair = nodeInfo.UsageGetRepair
	invoice.UsagePutRepair = nodeInfo.UsagePutRepair
	invoice.UsageGetAudit = nodeInfo.UsageGetAudit
	invoice.TotalHeld = nodeInfo.TotalHeld
	invoice.TotalDisposed = nodeInfo.TotalDisposed
	invoice.TotalPaid = nodeInfo.TotalPaid
	invoice.TotalDistributed = nodeInfo.TotalDistributed
	return nil
}

// MergeStatement updates the fields representing the calculation of the payment amounts
// into the invoice.
func (invoice *Invoice) MergeStatement(statement Statement) error {
	if invoice.NodeID != NodeID(statement.NodeID) {
		return Error.New("node ID mismatch (invoice=%q statement=%q)", invoice.NodeID, statement.NodeID)
	}
	invoice.Codes = statement.Codes
	invoice.CompAtRest = statement.AtRest
	invoice.CompGet = statement.Get
	invoice.CompPut = statement.Put
	invoice.CompGetRepair = statement.GetRepair
	invoice.CompPutRepair = statement.PutRepair
	invoice.CompGetAudit = statement.GetAudit
	invoice.SurgePercent = statement.SurgePercent
	invoice.Owed = statement.Owed
	invoice.Held = statement.Held
	invoice.Disposed = statement.Disposed
	return nil
}

// ReadInvoices reads a collection of Invoice values in CSV form.
func ReadInvoices(r io.Reader) ([]Invoice, error) {
	var invoices []Invoice
	if err := strictcsv.Read(r, &invoices); err != nil {
		return nil, err
	}
	return invoices, nil
}

// WriteInvoices writes a collection of Invoice values in CSV form.
func WriteInvoices(w io.Writer, invoices []Invoice) error {
	return strictcsv.Write(w, invoices)
}
