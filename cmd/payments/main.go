// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

var (
	ctx = context.Background()
	// ErrPaymentsDial throws when there are errors dialing the payments client
	ErrPaymentsDial = errs.Class("error dialing payments client:")

	// ErrRequest is for gRPC request errors after dialing
	ErrRequest = errs.Class("error processing request:")

	// ErrIdentity is for errors during identity creation for this CLI
	ErrIdentity = errs.Class("error creating identity:")

	// ErrArgs throws when there are errors with CLI args
	ErrArgs = errs.Class("error with CLI args:")

	port string
	// apiKey string

	rootCmd = &cobra.Command{Use: "payments"}

	cmdGenerate = &cobra.Command{
		Use:   "GenerateCSV",
		Short: "Generates payment csv",
		Args:  cobra.MinimumNArgs(2),
		RunE:  GenerateCSV,
	}
)

// Payments gives access to the payments api
type Payments struct {
	client pb.PaymentsClient
}

func main() {
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", ":7778", "satellite port")
	// rootCmd.PersistentFlags().StringVarP(&apiKey, "apikey", "a", "abc123", "satellite api key")
	rootCmd.AddCommand(cmdGenerate)
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
	conn, err := tc.DialAddress(ctx, port)
	if err != nil {
		return &Payments{}, ErrPaymentsDial.Wrap(err)
	}

	c := pb.NewPaymentsClient(conn)
	return &Payments{client: c}, nil
}

// GenerateCSV makes a call to the payments client to query the db and generate a csv
func GenerateCSV(cmd *cobra.Command, args []string) error {
	//TODO check validity of args
	p, err := NewPayments()
	startTime := args[0]
	endTime := args[1]
	start := time.Now()
	end := time.Now()

	p.client.GenerateCSV(ctx, start, end)
	// return query(args[0], args[1])
}
