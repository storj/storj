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

	"storj.io/common/storj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb"
)

func generateInvoicesCSV(ctx context.Context, period compensation.Period, out io.Writer) (err error) {
	periodInfo := compensation.PeriodInfo{
		Period: period,
		Rates: &compensation.Rates{
			AtRestGBHours: generateInvoicesCfg.Compensation.Rates.AtRestGBHours,
			GetTB:         generateInvoicesCfg.Compensation.Rates.GetTB,
			PutTB:         generateInvoicesCfg.Compensation.Rates.PutTB,
			GetRepairTB:   generateInvoicesCfg.Compensation.Rates.GetRepairTB,
			PutRepairTB:   generateInvoicesCfg.Compensation.Rates.PutRepairTB,
			GetAuditTB:    generateInvoicesCfg.Compensation.Rates.GetAuditTB,
		},
		SurgePercent:     generateInvoicesCfg.SurgePercent,
		DisposePercent:   generateInvoicesCfg.Compensation.DisposePercent,
		WithheldPercents: generateInvoicesCfg.Compensation.WithheldPercents,
	}

	db, err := satellitedb.Open(ctx, zap.L().Named("db"), generateInvoicesCfg.Database, satellitedb.Options{ApplicationName: "satellite-compensation"})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	if err := db.CheckVersion(ctx); err != nil {
		zap.L().Fatal("Failed satellite database version check.", zap.Error(err))
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	periodUsage, err := db.StoragenodeAccounting().QueryStorageNodePeriodUsage(ctx, period)
	if err != nil {
		return err
	}

	periodUsageByNode := make(map[storj.NodeID]accounting.StorageNodePeriodUsage, len(periodUsage))
	for _, usage := range periodUsage {
		periodUsageByNode[usage.NodeID] = usage
	}

	var allNodes []*overlay.NodeDossier
	err = db.OverlayCache().IterateAllNodeDossiers(ctx,
		func(ctx context.Context, node *overlay.NodeDossier) error {
			allNodes = append(allNodes, node)
			return nil
		})
	if err != nil {
		return err
	}

	invoices := make([]compensation.Invoice, 0, len(allNodes))
	for _, node := range allNodes {
		totalAmounts, err := db.Compensation().QueryTotalAmounts(ctx, node.Id)
		if err != nil {
			return err
		}

		var gracefulExit *time.Time
		if node.ExitStatus.ExitSuccess {
			gracefulExit = node.ExitStatus.ExitFinishedAt
		}
		nodeAddress, _, err := net.SplitHostPort(node.Address.Address)
		if err != nil {
			return errs.New("unable to split node %q address %q", node.Id, node.Address.Address)
		}
		var nodeLastIP string
		if node.LastIPPort != "" {
			nodeLastIP, _, err = net.SplitHostPort(node.LastIPPort)
			if err != nil {
				return errs.New("unable to split node %q last ip:port %q", node.Id, node.LastIPPort)
			}
		}

		// the zero value of period usage is acceptable for if the node does not have
		// any usage for the period.
		usage := periodUsageByNode[node.Id]
		nodeInfo := compensation.NodeInfo{
			ID:                 node.Id,
			CreatedAt:          node.CreatedAt,
			LastContactSuccess: node.Reputation.LastContactSuccess,
			Disqualified:       node.Disqualified,
			GracefulExit:       gracefulExit,
			UsageAtRest:        usage.AtRestTotal,
			UsageGet:           usage.GetTotal,
			UsagePut:           usage.PutTotal,
			UsageGetRepair:     usage.GetRepairTotal,
			UsagePutRepair:     usage.PutRepairTotal,
			UsageGetAudit:      usage.GetAuditTotal,
			TotalHeld:          totalAmounts.TotalHeld,
			TotalDisposed:      totalAmounts.TotalDisposed,
			TotalPaid:          totalAmounts.TotalPaid,
			TotalDistributed:   totalAmounts.TotalDistributed,
		}

		invoice := compensation.Invoice{
			Period:             period,
			NodeID:             compensation.NodeID(node.Id),
			NodeWallet:         node.Operator.Wallet,
			NodeWalletFeatures: node.Operator.WalletFeatures,
			NodeAddress:        nodeAddress,
			NodeLastIP:         nodeLastIP,
		}

		if err := invoice.MergeNodeInfo(nodeInfo); err != nil {
			return err
		}
		invoices = append(invoices, invoice)
		periodInfo.Nodes = append(periodInfo.Nodes, nodeInfo)
	}

	statements, err := compensation.GenerateStatements(periodInfo)
	if err != nil {
		return err
	}

	for i := 0; i < len(statements); i++ {
		if err := invoices[i].MergeStatement(statements[i]); err != nil {
			return err
		}
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

	db, err := satellitedb.Open(ctx, zap.L().Named("db"), recordPeriodCfg.Database, satellitedb.Options{ApplicationName: "satellite-compensation"})
	if err != nil {
		return 0, 0, errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	if err := db.CheckVersion(ctx); err != nil {
		zap.L().Fatal("Failed satellite database version check.", zap.Error(err))
		return 0, 0, errs.New("Error checking version for satellitedb: %+v", err)
	}

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

	db, err := satellitedb.Open(ctx, zap.L().Named("db"), recordOneOffPaymentsCfg.Database, satellitedb.Options{ApplicationName: "satellite-compensation"})
	if err != nil {
		return 0, errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() { err = errs.Combine(err, db.Close()) }()

	if err := db.CheckVersion(ctx); err != nil {
		zap.L().Fatal("Failed satellite database version check.", zap.Error(err))
		return 0, errs.New("Error checking version for satellitedb: %+v", err)
	}

	if err := db.Compensation().RecordPayments(ctx, payments); err != nil {
		return 0, err
	}

	return len(payments), nil
}
