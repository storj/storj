// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
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
	// rows, err := srv.accountingDB.QueryPaymentInfo(ctx, req.startTime, req.endTime)

	// headers := []string{"nodeID", "nodeIDCreationDate", "nodeStatus", "walletAddress", "GBAtRest", "GBBWRepair", "GBBWAudit", "GBBWDownload", "start", "end", "satelliteID"}
	// file, err := os.Create(srv.filepath + req.startTime + "-" + req.endTime + ".csv")
	// if err != nil {
	// 	return err
	// }
	// defer file.Close()

	// w := csv.NewWriter(file)
	// if err := w.Write(headers); err != nil {
	// 	log.Fatalln("error writing headers to csv:", err)
	// }

	// // qErr := query(startTime, endTime)
	// // if qErr != nil {
	// // 	return err
	// // }

	// if err := w.Error(); err != nil {
	// 	log.Fatal(err)
	// }
	// w.Flush()
	return &pb.GenerateCSVResponse{}, nil
}
