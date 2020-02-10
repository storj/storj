// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"

	"storj.io/storj/pkg/strictcsv"
	"storj.io/storj/private/currency"
)

type Invoice struct {
	Period           Period             `csv:"period"`
	NodeID           NodeID             `csv:"node-id"`
	NodeCreatedAt    UTCDate            `csv:"node-created-at"`
	NodeDisqualified *UTCDate           `csv:"node-disqualified"`
	NodeGracefulExit *UTCDate           `csv:"node-gracefulexit"`
	NodeWallet       string             `csv:"node-wallet"`
	NodeAddress      string             `csv:"node-address"`
	NodeLastIP       string             `csv:"node-last-ip"`
	Codes            Codes              `csv:"codes"`
	UsageAtRest      float64            `csv:"usage-at-rest"`
	UsageGet         int64              `csv:"usage-get"`
	UsagePut         int64              `csv:"usage-put"`
	UsageGetRepair   int64              `csv:"usage-get-repair"`
	UsagePutRepair   int64              `csv:"usage-put-repair"`
	UsageGetAudit    int64              `csv:"usage-get-audit"`
	CompAtRest       currency.MicroUnit `csv:"comp-at-rest"`
	CompGet          currency.MicroUnit `csv:"comp-get"`
	CompPut          currency.MicroUnit `csv:"comp-put"`
	CompGetRepair    currency.MicroUnit `csv:"comp-get-repair"`
	CompPutRepair    currency.MicroUnit `csv:"comp-put-repair"`
	CompGetAudit     currency.MicroUnit `csv:"comp-get-audit"`
	SurgePercent     int                `csv:"surge-percent"`
	Owed             currency.MicroUnit `csv:"owed"`
	Held             currency.MicroUnit `csv:"held"`
	Disposed         currency.MicroUnit `csv:"disposed"`
	TotalHeld        currency.MicroUnit `csv:"total-held"`
	TotalDisposed    currency.MicroUnit `csv:"total-disposed"`
	PayedYTD         currency.MicroUnit `csv:"payed-ytd"`
}

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
	return nil
}

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

func ReadInvoices(r io.Reader) ([]Invoice, error) {
	var invoices []Invoice
	if err := strictcsv.Read(r, &invoices); err != nil {
		return nil, err
	}
	return invoices, nil
}

func WriteInvoices(w io.Writer, invoices []Invoice) error {
	return strictcsv.Write(w, invoices)
}
