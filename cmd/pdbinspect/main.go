// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
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

			for index, pointer := range items {
				pointerFields := map[string]interface{}{
					"Index": index,
					"Path":  pointer.Path,
				}
				formatted, err := json.MarshalIndent(pointerFields, "", "  ")
				if err != nil {
					fmt.Println("error:", err)
				}
				fmt.Println(string(formatted))
			}
			if more {
				fmt.Println("\n\nMore pointers remaining.\nRun list again with `--startAfter <last-path>`.")
			}
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

			prettyPointer := prettyPrint(pointer)
			fmt.Println(prettyPointer)

			prettyPBA := prettyPrint(pba)
			fmt.Println(prettyPBA)

			for index, node := range nodes {
				prettyNode := prettyPrint(node)
				fmt.Print(index, prettyNode)
			}
		},
	}

	cmdList.Flags().StringVarP(&prefix, "prefix", "x", "", "bucket prefix")
	cmdList.Flags().StringVarP(&endBefore, "endBefore", "e", "", "end before path")
	cmdList.Flags().StringVarP(&startAfter, "startAfter", "s", "", "start after path")
	cmdList.Flags().BoolVarP(&recursive, "recursive", "r", true, "recursively list")
	cmdList.Flags().IntVarP(&limit, "limit", "l", 0, "listing limit")
	cmdList.Flags().Uint32VarP(&metaFlags, "metaFlags", "m", meta.None, "listing limit")

	var rootCmd = &cobra.Command{Use: "pdbinspect"}
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", ":7778", "pointerdb port")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "apikey", "a", "abc123", "pointerdb api key")

	rootCmd.AddCommand(cmdList, cmdGet)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
}

func prettyPrint(unformatted proto.Message) string {
	m := jsonpb.Marshaler{Indent: "  ", EmitDefaults: true}
	formatted, err := m.MarshalToString(unformatted)
	if err != nil {
		fmt.Println("Error", err)
	}
	return formatted
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
