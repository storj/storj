// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storage/meta"
)

func main() {
	ctx := context.Background()
	var port string
	var apiKey string
	var prefix string
	var endBefore string
	var startAfter string
	var recursive bool
	var limit int
	var metaFlags uint32

	var cmdList = &cobra.Command{
		Use:   "list",
		Short: "lists pointers",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newPdbClient(ctx, port, apiKey)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			items, more, err := client.List(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

			fmt.Println("Pointers ----------------")

			for index, pointer := range items {
				pointerFields := map[string]interface{}{
					"Index":           index,
					"Path":            pointer.Path,
					"IsPrefix":        pointer.IsPrefix,
					"Type":            pointer.Pointer.GetType(),
					"Remote":          pointer.Pointer.GetRemote(),
					"Segment Size":    pointer.Pointer.GetSegmentSize(),
					"Creation Date":   pointer.Pointer.GetCreationDate(),
					"Expiration Date": pointer.Pointer.GetExpirationDate(),
				}
				formatted, err := json.MarshalIndent(pointerFields, "", "  ")
				if err != nil {
					fmt.Println("error:", err)
				}
				fmt.Print(string(formatted))
			}
			fmt.Println("\n\nMore pointers remaining:", more)
		},
	}

	var cmdGet = &cobra.Command{
		Use:   "get",
		Short: "gets pointer",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := newPdbClient(ctx, port, apiKey)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			pointer, nodes, pba, err := client.Get(ctx, args[0])
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

			pointerFields := map[string]interface{}{
				"Type":            pointer.GetType(),
				"Redundancy":      pointer.GetRemote().GetRedundancy(),
				"Segment Size":    pointer.GetSegmentSize(),
				"Creation Date":   pointer.GetCreationDate(),
				"Expiration Date": pointer.GetExpirationDate(),
				"PBA Signature":   pba.Signature,
				"PBA Data":        pba.Data,
			}
			formatted, err := json.MarshalIndent(pointerFields, "", "  ")
			if err != nil {
				fmt.Println("error:", err)
			}
			fmt.Print(string(formatted))

			for index, node := range nodes {
				nodeFields := map[string]interface{}{
					"Index":         index,
					"NodeID":        node.Id,
					"Address":       node.GetAddress(),
					"Audit Success": node.GetAuditSuccess(),
					"Is Up":         node.GetIsUp(),
					"Latency List":  node.GetLatencyList(),
					"Metadata":      node.GetMetadata(),
					"Reputation":    node.GetReputation(),
					"Restrictions":  node.GetRestrictions(),
					"Type":          node.GetType(),
				}
				formatted, err := json.MarshalIndent(nodeFields, "", "  ")
				if err != nil {
					fmt.Println("error:", err)
				}
				fmt.Print(string(formatted))
			}
		},
	}

	cmdList.Flags().StringVarP(&port, "port", "p", ":7778", "pointerdb port")
	cmdList.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "pointerdb api key")
	cmdList.Flags().StringVarP(&prefix, "prefix", "x", "", "bucket prefix")
	cmdList.Flags().StringVarP(&endBefore, "endBefore", "e", "", "end before path")
	cmdList.Flags().StringVarP(&startAfter, "startAfter", "s", "", "start after path")
	cmdList.Flags().BoolVarP(&recursive, "recursive", "r", true, "recursively list")
	cmdList.Flags().IntVarP(&limit, "limit", "l", 0, "listing limit")
	cmdList.Flags().Uint32VarP(&metaFlags, "metaFlags", "m", meta.None, "listing limit")

	cmdGet.Flags().StringVarP(&port, "port", "p", ":7778", "pointerdb port")
	cmdGet.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "pointerdb api key")

	var rootCmd = &cobra.Command{Use: "pdbinspect"}
	rootCmd.AddCommand(cmdList, cmdGet)
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
