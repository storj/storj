// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/pb"
)

var (
	ctx = context.Background()

	port   string
	apiKey string

	rootCmd = &cobra.Command{Use: "payments"}

	cmdGenerate = &cobra.Command{
		Use:   "generateCSV",
		Short: "generates payment csv",
		Args:  cobra.MinimumNArgs(2),
		RunE:  generateCSV,
	}
)

// Payments gives access to the payments api
type Payments struct {
	client pb.PaymentsClient
}

func main() {
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", ":7778", "satellite port")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "apikey", "a", "abc123", "satellite api key")
	rootCmd.AddCommand(cmdGenerate)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
}

func NewPayments(address string) (*Payments, error) {
	ctx := context.Background()

}

// func NewInspector(address string) (*Inspector, error) {
// 	ctx := context.Background()
// 	identity, err := provider.NewFullIdentity(ctx, 12, 4)
// 	if err != nil {
// 		return &Inspector{}, ErrIdentity.Wrap(err)
// 	}

// 	tc := transport.NewClient(identity)
// 	conn, err := tc.DialAddress(ctx, address)
// 	if err != nil {
// 		return &Inspector{}, ErrInspectorDial.Wrap(err)
// 	}

// 	c := pb.NewInspectorClient(conn)

// 	return &Inspector{
// 		identity: identity,
// 		client:   c,
// 	}, nil
// }


func generateCSV(cmd *cobra.Command, args []string) error {
	//TODO check validity of args

	startTime := args[0]
	endTime := args[1]
	pc := pb.NewPaymentsClient()
	// return query(args[0], args[1])
}
