// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/satellitedb"
)

func generateInvoicesCSV(ctx context.Context, period compensation.Period, out io.Writer) (err error) {
	periodInfo := compensation.PeriodInfo{
		Period: period,
		Rates: &compensation.Rates{
			AtRestGBHours: generateInvoicesCfg.Comp.Rates.AtRestGBHours,
			GetTB:         generateInvoicesCfg.Comp.Rates.GetTB,
			PutTB:         generateInvoicesCfg.Comp.Rates.PutTB,
			GetRepairTB:   generateInvoicesCfg.Comp.Rates.GetRepairTB,
			PutRepairTB:   generateInvoicesCfg.Comp.Rates.PutRepairTB,
			GetAuditTB:    generateInvoicesCfg.Comp.Rates.GetAuditTB,
		},
		SurgePercent:   generateInvoicesCfg.SurgePercent,
		DisposePercent: generateInvoicesCfg.Comp.DisposePercent,
		EscrowPercents: generateInvoicesCfg.Comp.EscrowPercents,
	}

	db, err := satellitedb.New(zap.L().Named("db"), generateInvoicesCfg.Database, satellitedb.Options{})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	periodUsage, err := db.StoragenodeAccounting().QueryStorageNodePeriodUsage(ctx, period)
	if err != nil {
		return err
	}

	invoices := make([]compensation.Invoice, 0, len(periodUsage))
	for _, usage := range periodUsage {
		escrowAmounts, err := db.Compensation().QueryEscrowAmounts(ctx, usage.NodeID)
		if err != nil {
			return err
		}

		node, err := db.OverlayCache().Get(ctx, usage.NodeID)
		if err != nil {
			return err
		}
		var gracefulExit *time.Time
		if node.ExitStatus.ExitSuccess {
			gracefulExit = node.ExitStatus.ExitFinishedAt
		}
		//var nodeAddress string
		//if node.Address != nil && node.Address.Address != "" {
		//	nodeAddress, _, err = net.SplitHostPort(node.Address.Address)
		nodeAddress, _, err := net.SplitHostPort(node.Address.Address)
		if err != nil {
			return errs.New("unable to split node %q address %q", usage.NodeID, node.Address.Address)
		}
		//}

		payedYTD, err := db.Compensation().QueryPayedInYear(ctx, usage.NodeID, period.Year)
		if err != nil {
			return err
		}

		nodeInfo := compensation.NodeInfo{
			ID:             usage.NodeID,
			CreatedAt:      node.CreatedAt,
			Disqualified:   node.Disqualified,
			GracefulExit:   gracefulExit,
			UsageAtRest:    usage.AtRestTotal,
			UsageGet:       usage.GetTotal,
			UsagePut:       usage.PutTotal,
			UsageGetRepair: usage.GetRepairTotal,
			UsagePutRepair: usage.PutRepairTotal,
			UsageGetAudit:  usage.GetAuditTotal,
			TotalHeld:      escrowAmounts.TotalHeld,
			TotalDisposed:  escrowAmounts.TotalDisposed,
		}

		invoice := compensation.Invoice{
			Period:      period,
			NodeID:      compensation.NodeID(usage.NodeID),
			NodeWallet:  node.Operator.Wallet,
			NodeAddress: nodeAddress,
			NodeLastIP:  node.LastIp,
			PayedYTD:    payedYTD,
		}

		invoice.MergeNodeInfo(nodeInfo)
		invoices = append(invoices, invoice)
		periodInfo.Nodes = append(periodInfo.Nodes, nodeInfo)
	}

	statements, err := compensation.GenerateStatements(periodInfo)
	if err != nil {
		return err
	}

	for i := 0; i < len(statements); i++ {
		invoices[i].MergeStatement(statements[i])
	}

	if err := compensation.WriteInvoices(out, invoices); err != nil {
		return err
	}

	return nil
}

func recordPeriod(ctx context.Context, paystubsCSV, paymentsCSV string) (int, int, error) {
	paystubs, err := compensation.LoadPaystubs(paystubsCSV)
	if err != nil {
		return 0, 0, err
	}

	payments, err := compensation.LoadPayments(paymentsCSV)
	if err != nil {
		return 0, 0, err
	}

	db, err := satellitedb.New(zap.L().Named("db"), recordPeriodCfg.Database, satellitedb.Options{})
	if err != nil {
		return 0, 0, errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	if err := db.Compensation().RecordPeriod(ctx, paystubs, payments); err != nil {
		return 0, 0, err
	}

	return len(paystubs), len(payments), nil
}

func recordOneOffPayments(ctx context.Context, paymentsCSV string) (int, error) {
	payments, err := compensation.LoadPayments(paymentsCSV)
	if err != nil {
		return 0, err
	}

	db, err := satellitedb.New(zap.L().Named("db"), recordOneOffPaymentsCfg.Database, satellitedb.Options{})
	if err != nil {
		return 0, errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	if err := db.Compensation().RecordPayments(ctx, payments); err != nil {
		return 0, err
	}

	return len(payments), nil
}

func dateToString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}
