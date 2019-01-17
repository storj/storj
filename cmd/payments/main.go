// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

var (
	ctx = context.Background()
	// ErrPaymentsDial throws when there are errors dialing the payments client
	ErrPaymentsDial = errs.Class("error dialing payments client")

	// ErrRequest is for gRPC request errors after dialing
	ErrRequest = errs.Class("error processing request")

	// ErrIdentity is for errors during identity creation for this CLI
	ErrIdentity = errs.Class("error creating identity")

	// ErrArgs throws when there are errors with CLI args
	ErrArgs = errs.Class("error with CLI args")

	port string

	rootCmd = &cobra.Command{Use: "payments"}

	cmdGenerate = &cobra.Command{
		Use:   "generatecsv",
		Short: "generates payment csv",
		Args:  cobra.MinimumNArgs(2),
		RunE:  generatecsv,
	}

	cmdTest = &cobra.Command{
		Use:   "test",
		Short: "test only: add records to accounting rollup",
		RunE:  test,
	}
)

// Payments gives access to the payments api
type Payments struct {
	client pb.PaymentsClient
}

func main() {
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", ":10000", "satellite port")
	rootCmd.AddCommand(cmdGenerate)
	rootCmd.AddCommand(cmdTest)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
}

// NewPayments creates a payments object
func NewPayments() (*Payments, error) {
	identity, err := provider.NewFullIdentity(ctx, 12, 4)
	if err != nil {
		return &Payments{}, ErrIdentity.Wrap(err)
	}
	tc := transport.NewClient(identity)

	// TODO: this might be blocked after requiring authorization
	// use satellite identity or later private grpc
	fmt.Println("Warning: created new ID, may not be able to connect to satellite")
	conn, err := tc.DialAddress(ctx, port)
	if err != nil {
		return &Payments{}, ErrPaymentsDial.Wrap(err)
	}

	c := pb.NewPaymentsClient(conn)
	return &Payments{client: c}, nil
}

// generateCSV makes a call to the payments client to query the db and generate a csv
func generatecsv(cmd *cobra.Command, args []string) error {
	fmt.Println("entering payments generatecsv")
	layout := "2006-01-02"
	start, err := time.Parse(layout, args[0])
	if err != nil {
		return ErrArgs.Wrap(errs.New("Invalid date format. Please use YYYY-MM-DD"))
	}
	end, err := time.Parse(layout, args[1])
	if err != nil {
		return ErrArgs.Wrap(errs.New("Invalid date format. Please use YYYY-MM-DD"))
	}

	// Ensure that start date is not after end date
	if start.After(end) {
		return errs.New("Invalid time period (%v) - (%v)", start, end)
	}

	startTimestamp, err := ptypes.TimestampProto(start)
	if err != nil {
		return err
	}
	endTimestamp, err := ptypes.TimestampProto(end)
	if err != nil {
		return err
	}
	p, err := NewPayments()
	if err != nil {
		return err
	}

	req := &pb.GenerateCSVRequest{
		StartTime: startTimestamp,
		EndTime:   endTimestamp,
	}

	resp, err := p.client.GenerateCSV(ctx, req)
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Println("Created payments report at", resp.GetFilepath())
	return nil
}

// test only: add records to accounting rollup
func test(cmd *cobra.Command, args []string) error {
	p, err := NewPayments()
	if err != nil {
		return err
	}
	req := &pb.TestRequest{}
	_, err = p.client.Test(ctx, req)
	fmt.Println("Added nodes to accounting rollup")
	return err
}
