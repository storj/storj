// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
)

func main() {
	ctx := context.Background()
	var port string
	var apiKey string

	var cmdList = &cobra.Command{
		Use:   "list",
		Short: "lists pointers",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newPdbClient(ctx, port, apiKey)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			items, more, err := client.List(ctx, "", "", "", true, 100, 0)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			fmt.Println(items)
			fmt.Println(more)
			fmt.Println("success!")
		},
	}

	cmdList.Flags().StringVarP(&port, "port", "p", ":7778", "pointerdb port")
	cmdList.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "pointerdb api key")

	var rootCmd = &cobra.Command{Use: "pdbinspect"}
	rootCmd.AddCommand(cmdList)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
}

// newPdbClient creates a new pointerdb client
func newPdbClient(ctx context.Context, port, apiKey string) (*pdbclient.PointerDB, error) {
	identity, err := provider.NewFullIdentity(ctx, 12, 4)
	if err != nil {
		return nil, err
	}
	client, err := pdbclient.NewClient(identity, port, apiKey)
	if err != nil {
		return nil, err
	}
	return client, nil
}
