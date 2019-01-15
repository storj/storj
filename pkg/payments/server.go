// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"encoding/csv"
	"os"
	"strconv"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

var (
	// PaymentsError is a gRPC server error for Payments
	PaymentsError = errs.Class("payments server error: ")
)

// Server holds references to ...
type Server struct {
	filepath     string
	accountingDB accounting.DB
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

	file, err := os.Create(srv.filepath + pi.ID.String() + ":" + start.String() + "-" + end.String() + ".csv")
	if err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}
	defer file.Close()

	rows, err := srv.accountingDB.QueryPaymentInfo(ctx, start, end)
	if err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}

	w := csv.NewWriter(file)
	for _, record := range rows {
		r := []string{string(record.Node_Id), record.Node_CreatedAt.String(), strconv.FormatFloat(record.Node_AuditSuccessRatio, 'f', 5, 64), record.AccountingRollup_DataType, string(record.AccountingRollup_DataTotal), record.AccountingRollup_CreatedAt.String()}
		if err := w.Write(r); err != nil {
			return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
		}
	}
	if err := w.Error(); err != nil {
		return &pb.GenerateCSVResponse{}, PaymentsError.Wrap(err)
	}
	w.Flush()
	return &pb.GenerateCSVResponse{}, nil
}
