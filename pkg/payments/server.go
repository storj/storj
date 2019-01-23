// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
)

var (
	mon = monkit.Package()
	// Error is the main payments error class for this package
	Error = errs.Class("payments server error: ")
)

// Config is a configuration struct for everything you need to start the
// Payments responsibility.
type Config struct {
	Filepath string `help:"the file path of the generated csv" default:"$CONFDIR/payments"`
	// TODO: service should not write to disk, but return the result instead
}

// Server holds references to info needed for the payments responsibility
type Server struct { // TODO: separate endpoint and service
	filepath     string
	accountingDB accounting.DB
	overlayDB    overlay.DB
	log          *zap.Logger
}

// New creates a new payments Endpoint
func New(log *zap.Logger, accounting accounting.DB, overlay overlay.DB, filepath string) *Server {
	return &Server{
		filepath:     filepath,
		accountingDB: accounting,
		overlayDB:    overlay,
		log:          log,
	}
}

// Pay creates a payment to a single storage node
func (srv *Server) Pay(ctx context.Context, req *pb.PaymentRequest) (*pb.PaymentResponse, error) {
	// TODO
	return &pb.PaymentResponse{}, Error.New("Pay not implemented")
}

// Calculate determines the outstanding balance for a given storage node
func (srv *Server) Calculate(ctx context.Context, req *pb.CalculateRequest) (*pb.CalculateResponse, error) {
	// TODO
	return &pb.CalculateResponse{}, Error.New("Calculate not implemented")
}

// AdjustPrices sets the prices paid by a satellite for data at rest and bandwidth
func (srv *Server) AdjustPrices(ctx context.Context, req *pb.AdjustPricesRequest) (*pb.AdjustPricesResponse, error) {
	// TODO
	return &pb.AdjustPricesResponse{}, Error.New("AdjustPrices not implemented")
}

// GenerateCSV creates a csv file for payment purposes
func (srv *Server) GenerateCSV(ctx context.Context, req *pb.GenerateCSVRequest) (_ *pb.GenerateCSVResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	start, err := ptypes.Timestamp(req.StartTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	end, err := ptypes.Timestamp(req.EndTime)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pi, err := provider.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	layout := "2006-01-02"

	if err := os.MkdirAll(srv.filepath, 0700); err != nil {
		return nil, Error.Wrap(err)
	}

	filename := pi.ID.String() + "--" + start.Format(layout) + "--" + end.Format(layout) + ".csv"
	path := filepath.Join(srv.filepath, filename)
	file, err := os.Create(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer utils.LogClose(file)

	rows, err := srv.accountingDB.QueryPaymentInfo(ctx, start, end)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	w := csv.NewWriter(file)
	headers := []string{
		"nodeID",
		"nodeCreationDate",
		"auditSuccessRatio",
		"byte-hours:AtRest",
		"bytes:BWRepair-GET",
		"bytes:BWRepair-PUT",
		"bytes:BWAudit",
		"bytes:BWGet",
		"bytes:BWPut",
		"date",
		"walletAddress",
	}
	if err := w.Write(headers); err != nil {
		return nil, Error.Wrap(err)
	}

	for _, row := range rows {
		nid := row.NodeID
		wallet, err := srv.overlayDB.GetWalletAddress(ctx, nid)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		row.Wallet = wallet
		record := structToStringSlice(row)
		if err := w.Write(record); err != nil {
			return nil, Error.Wrap(err)
		}
	}
	if err := w.Error(); err != nil {
		return nil, Error.Wrap(err)
	}
	w.Flush()
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return &pb.GenerateCSVResponse{Filepath: abs}, nil
}

// Test TODO: remove
func (srv *Server) Test(ctx context.Context, req *pb.TestRequest) (*pb.TestResponse, error) {
	err := srv.accountingDB.TestPayments(ctx)
	return &pb.TestResponse{}, err
}

func structToStringSlice(s *accounting.CSVRow) []string {
	record := []string{
		s.NodeID.String(),
		s.NodeCreationDate.Format("2006-01-02"),
		strconv.FormatFloat(s.AuditSuccessRatio, 'f', 5, 64),
		strconv.FormatFloat(s.AtRestTotal, 'f', 5, 64),
		strconv.FormatInt(s.GetRepairTotal, 10),
		strconv.FormatInt(s.PutRepairTotal, 10),
		strconv.FormatInt(s.GetAuditTotal, 10),
		strconv.FormatInt(s.PutTotal, 10),
		strconv.FormatInt(s.GetTotal, 10),
		s.Date.Format("2006-01-02"),
		s.Wallet,
	}
	return record
}
