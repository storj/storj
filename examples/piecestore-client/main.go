// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/provider"
)

var ctx = context.Background()
var argError = errs.Class("argError")

func main() {
	cobra.EnableCommandSorting = false

	ca, err := provider.NewCA(ctx, 12, 4)
	if err != nil {
		log.Fatal(err)
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		log.Fatal(err)
	}
	identOpt, err := identity.DialOption()
	if err != nil {
		log.Fatal(err)
	}

	// Set up connection with rpc server
	var conn *grpc.ClientConn
	conn, err = grpc.Dial(":7777", identOpt)
	if err != nil {
		log.Fatalf("did not connect: %s", err)
	}
	defer printError(conn.Close)

	psClient, err := client.NewPSClient(conn, 1024*32, identity.Key.(*ecdsa.PrivateKey))
	if err != nil {
		log.Fatalf("could not initialize PSClient: %s", err)
	}

	root := &cobra.Command{
		Use:   "piecestore-client",
		Short: "piecestore example client",
	}

	root.AddCommand(&cobra.Command{
		Use:     "upload [input-file]",
		Short:   "Upload data",
		Aliases: []string{"u"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputfile := args[0]

			file, err := os.Open(inputfile)
			if err != nil {
				return err
			}
			// Close the file when we are done
			defer printError(file.Close)

			fileInfo, err := file.Stat()
			if err != nil {
				return err
			}

			if fileInfo.IsDir() {
				return argError.New(fmt.Sprintf("path (%s) is a directory, not a file", inputfile))
			}

			var length = fileInfo.Size()
			var ttl = time.Now().Add(24 * time.Hour)

			// Created a section reader so that we can concurrently retrieve the same file.
			dataSection := io.NewSectionReader(file, 0, length)

			id := client.NewPieceID()

			if err := psClient.Put(context.Background(), id, dataSection, ttl, &pb.PayerBandwidthAllocation{}); err != nil {
				fmt.Printf("Failed to Store data of id: %s\n", id)
				return err
			}

			fmt.Printf("Successfully stored file of id: %s\n", id)

			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "download [id] [output-dir]",
		Short:   "Download data",
		Aliases: []string{"d"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			outputDir := args[1]

			_, err := os.Stat(outputDir)
			if err != nil && !os.IsNotExist(err) {
				return err
			}

			if err == nil {
				return argError.New("File already exists")
			}

			if err = os.MkdirAll(filepath.Dir(outputDir), 0700); err != nil {
				return err
			}

			// Create File on file system
			dataFile, err := os.OpenFile(outputDir, os.O_RDWR|os.O_CREATE, 0755)
			if err != nil {
				return err
			}
			defer printError(dataFile.Close)

			pieceInfo, err := psClient.Meta(context.Background(), client.PieceID(id))
			if err != nil {
				errRemove := os.Remove(outputDir)
				if errRemove != nil {
					log.Println(errRemove)
				}
				return err
			}

			rr, err := psClient.Get(ctx, client.PieceID(id), pieceInfo.Size, &pb.PayerBandwidthAllocation{})
			if err != nil {
				fmt.Printf("Failed to retrieve file of id: %s\n", id)
				errRemove := os.Remove(outputDir)
				if errRemove != nil {
					log.Println(errRemove)
				}
				return err
			}

			reader, err := rr.Range(ctx, 0, pieceInfo.Size)
			if err != nil {
				fmt.Printf("Failed to retrieve file of id: %s\n", id)
				errRemove := os.Remove(outputDir)
				if errRemove != nil {
					log.Println(errRemove)
				}
				return err
			}

			_, err = io.Copy(dataFile, reader)
			if err != nil {
				fmt.Printf("Failed to retrieve file of id: %s\n", id)
				errRemove := os.Remove(outputDir)
				if errRemove != nil {
					log.Println(errRemove)
				}
			} else {
				fmt.Printf("Successfully retrieved file of id: %s\n", id)
			}

			return reader.Close()
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "delete [id]",
		Short:   "Delete data",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"x"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return psClient.Delete(context.Background(), client.PieceID(id))
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "stat",
		Aliases: []string{"s"},
		Short:   "Retrieve statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			var summary *pb.StatSummary
			summary, err := psClient.Stats(context.Background())
			if err != nil {
				return err
			}

			log.Printf("Space Used: %v, Space Available: %v\n", summary.UsedSpace, summary.AvailableSpace)
			return nil
		},
	})

	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

func printError(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}
