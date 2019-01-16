// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/utils"
)

var (
	// PaymentsError is a gRPC server error for Payments
	PaymentsError = errs.Class("payments server error: ")
)

// Server holds references to ...
type Server struct {
	filepath     string
	accountingDB accounting.DB
	overlayDB    overlay.DB
	log          *zap.Logger
	metrics      *monkit.Registry
}

// Pay creates a payment to a single storage node
func (srv *Server) Pay(ctx context.Context, req *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	// TODO
	return &pb.PaymentResponse{}, PaymentsError.New("Pay not implemented")
}

// Calculate determines the outstanding balance for a given storage node
func (srv *Server) Calculate(ctx context.Context, req *pb.CalculateRequest) (*pb.CalculateResponse, error) {
	// TODO
	return &pb.CalculateResponse{}, PaymentsError.New("Calculate not implemented")
}

// AdjustPrices sets the prices paid by a satellite for data at rest and bandwidth
func (srv *Server) AdjustPrices(ctx context.Context, req *pb.AdjustPricesRequest) (*pb.AdjustPricesResponse, error) {
	// TODO
	return &pb.AdjustPricesResponse{}, PaymentsError.New("AdjustPrices not implemented")
}

// GenerateCSV creates a csv file for payment purposes
func (srv *Server) GenerateCSV(ctx context.Context, req *pb.GenerateCSVRequest) (*pb.GenerateCSVResponse, error) {
	fmt.Println("entering server generate csv")
	start, err := ptypes.Timestamp(req.StartTime)
	if err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}

	end, err := ptypes.Timestamp(req.EndTime)
	if err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}

	pi, err := provider.PeerIdentityFromContext(ctx)
	if err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}

	layout := "2006-01-02"

	if err := os.MkdirAll(srv.filepath, 0700); err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}

	filename := pi.ID.String() + "--" + start.Format(layout) + "--" + end.Format(layout) + ".csv"
	path := filepath.Join(srv.filepath, filename)
	file, err := os.Create(path)
	if err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}
	defer utils.LogClose(file)

	rows, err := srv.accountingDB.QueryPaymentInfo(ctx, start, end)
	if err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}

	w := csv.NewWriter(file)
	headers := []string{
		"nodeID",
		"nodeCreationDate",
		"auditSuccessRatio",
		"byte/hr:AtRest",
		"byte/hr:BWRepair-GET",
		"byte/hr:BWRepair-PUT",
		"byte/hr:BWAudit",
		"byte/hr:BWGet",
		"byte/hr:BWPut",
		"date",
		"walletAddress",
	}
	if err := w.Write(headers); err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}
	for _, record := range rows {
		if len([]byte(record[0])) != 32 {
			continue
		}
		nid, err := storj.NodeIDFromBytes([]byte(record[0]))
		if err != nil {
			return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
		}

		wallet, err := srv.overlayDB.GetWalletAddress(ctx, nid) //breaking
		if err != nil {
			return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
		}

		record = append(record, wallet)

		if err := w.Write(record); err != nil {
			return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
		}
	}
	if err := w.Error(); err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}
	w.Flush()
	abs, err := filepath.Abs(path)
	if err != nil {
		return &pb.GenerateCSVResponse{}, err
	}
	return &pb.GenerateCSVResponse{Filepath: abs}, nil
}

// Test TODO: remove
func (srv *Server) Test(ctx context.Context, req *pb.TestRequest) (*pb.TestResponse, error) {
	err := srv.accountingDB.TestPayments(ctx)
	return &pb.TestResponse{}, err
}