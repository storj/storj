// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storage/meta"
)

var (
	ctx = context.Background()

	port      string
	apiKey    string
	jsonPrint bool

	prefix     string
	endBefore  string
	startAfter string
	recursive  bool
	limit      int
	metaFlags  uint32

	rootCmd = &cobra.Command{Use: "pdbinspect"}

	cmdList = &cobra.Command{
		Use:   "list",
		Short: "lists pointers",
		Run:   listPointers,
	}

	cmdGet = &cobra.Command{
		Use:   "get",
		Short: "gets pointer",
		Args:  cobra.MinimumNArgs(1),
		Run:   getPointer,
	}
)

func main() {
	cmdList.Flags().StringVarP(&prefix, "prefix", "x", "", "bucket prefix")
	cmdList.Flags().StringVarP(&endBefore, "endBefore", "e", "", "end before path")
	cmdList.Flags().StringVarP(&startAfter, "startAfter", "s", "", "start after path")
	cmdList.Flags().BoolVarP(&recursive, "recursive", "r", true, "recursively list")
	cmdList.Flags().IntVarP(&limit, "limit", "l", 0, "listing limit")
	cmdList.Flags().Uint32VarP(&metaFlags, "metaFlags", "m", meta.None, "listing limit")

	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", ":7778", "pointerdb port")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "apikey", "a", "abc123", "pointerdb api key")
	rootCmd.PersistentFlags().BoolVarP(&jsonPrint, "json", "j", false, "formats in json")

	rootCmd.AddCommand(cmdList, cmdGet)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
}

func listPointers(cmd *cobra.Command, args []string) {
	client, err := newPDBClient(ctx, port, apiKey)
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
	items, more, err := client.List(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags)
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
	if jsonPrint {
		for index, pointer := range items {
			pointerFields := map[string]interface{}{
				"Index": index,
				"Path":  pointer.Path,
			}
			formatted, err := json.MarshalIndent(pointerFields, "", "  ")
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			fmt.Println(string(formatted))
		}
	} else {
		fmt.Println("Index\tPath\t")

		for i, pointer := range items {
			fmt.Println(i, "\t", pointer.Path, "\t")
		}
	}

	if more {
		fmt.Println("\nMore pointers remaining.\nRun list again with `--startAfter <last-path>`")
	}
}

func getPointer(cmd *cobra.Command, args []string) {
	client, err := newPDBClient(ctx, port, apiKey)
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
	pointer, _, _, err := client.Get(ctx, args[0])
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}

	if jsonPrint {
		prettyPointer := prettyPrint(pointer)
		fmt.Println(prettyPointer)
	} else {

		if pointer.GetType() == pb.Pointer_INLINE {
			fmt.Println("Type\tCreation Date\tExpiration Date\t")
			fmt.Println(pointer.GetType(), "\t", readableTime(pointer.GetCreationDate()), "\t",
				readableTime(pointer.GetExpirationDate()), "\t")
		}

		if pointer.GetType() == pb.Pointer_REMOTE {
			fmt.Println("\nRemote Pieces:")
			fmt.Println("\nIndex\tPiece Number\tNode ID")
			for index, piece := range pointer.GetRemote().GetRemotePieces() {
				fmt.Println(index, "\t", piece.GetPieceNum(), "\t\t", piece.NodeId)
			}

			fmt.Println("\nType\t\t", pointer.GetType(), "\nCreation Date\t", readableTime(pointer.GetCreationDate()),
				"\nExpiration Date\t", readableTime(pointer.GetExpirationDate()), "\nSegment Size\t", pointer.GetSegmentSize(),
				"\nPiece ID\t", pointer.GetRemote().GetPieceId(),
				"\n\nRedundancy:\n\n\tMinimum Required\t", pointer.GetRemote().GetRedundancy().GetMinReq(),
				"\n\tTotal\t\t\t", pointer.GetRemote().GetRedundancy().GetTotal(),
				"\n\tRepair Threshold\t", pointer.GetRemote().GetRedundancy().GetRepairThreshold(),
				"\n\tSuccess Threshold\t", pointer.GetRemote().GetRedundancy().GetSuccessThreshold(),
				"\n\tErasure Share Size\t", pointer.GetRemote().GetRedundancy().GetErasureShareSize(),
			)
		}
	}
}

func readableTime(stamp *timestamp.Timestamp) string {
	t := time.Unix(stamp.GetSeconds(), int64(stamp.GetNanos()))
	readable := t.Format(time.RFC822Z)
	return readable
}

func prettyPrint(unformatted proto.Message) string {
	m := jsonpb.Marshaler{Indent: "  ", EmitDefaults: true}
	formatted, err := m.MarshalToString(unformatted)
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
	return formatted
}

func newPDBClient(ctx context.Context, port, apiKey string) (*pdbclient.PointerDB, error) {
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
