// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli"
	"github.com/zeebo/errs"

	"golang.org/x/net/context"

	"google.golang.org/grpc"

	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/examples/piecestore/rpc/client/utils"
	pb "storj.io/storj/protos/piecestore"
)

var argError = errs.Class("argError")

func main() {

	app := cli.NewApp()

	// Set up connection with rpc server
	var conn *grpc.ClientConn
	conn, err := grpc.Dial(":7777", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %s", err)
	}
	defer conn.Close()
	routesClient := pb.NewPieceStoreRoutesClient(conn)
	ctx := context.Background()

	app.Commands = []cli.Command{
		{
			Name:    "upload",
			Aliases: []string{"u"},
			Usage:   "upload data",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return argError.New("No input file specified")
				}

				file, err := os.Open(c.Args().Get(0))
				if err != nil {
					return err
				}
				// Close the file when we are done
				defer file.Close()

				fileInfo, err := file.Stat()
				if err != nil {
					return err
				}

				if fileInfo.IsDir() {
					return argError.New(fmt.Sprintf("Path (%s) is a directory, not a file", c.Args().Get(0)))
				}

				var fileOffset, storeOffset int64 = 0, 0
				var length = fileInfo.Size()
				var ttl = time.Now().Unix() + 86400

				id, err := utils.DetermineID(file, fileOffset, length)
				if err != nil {
					return err
				}

				// Created a section reader so that we can concurrently retrieve the same file.
				dataSection := io.NewSectionReader(file, fileOffset, length)

				writer, err := client.StorePieceRequest(ctx, routesClient, id, fileOffset, length, ttl, storeOffset)
				if err != nil {
					fmt.Printf("Failed to send meta data to server to store file of id: %s\n", id)
					return err
				}
				defer writer.Close()

				_, err = io.Copy(writer, dataSection)
				if err != nil {
					fmt.Printf("Failed to store file of id: %s\n", id)
					return err
				}

				fmt.Printf("successfully Stored file of id: %s\n", id)
				return nil
			},
		},
		{
			Name:    "download",
			Aliases: []string{"d"},
			Usage:   "download data",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return argError.New("No id specified")
				}

				id := c.Args().Get(0)

				if c.Args().Get(1) == "" {
					return argError.New("No output file specified")
				}

				_, err := os.Stat(c.Args().Get(1))
				if err != nil && !os.IsNotExist(err) {
					return err
				}

				dataPath := c.Args().Get(1)

				if err = os.MkdirAll(filepath.Dir(dataPath), 0700); err != nil {
					return err
				}

				// Create File on file system
				dataFile, err := os.OpenFile(dataPath, os.O_RDWR|os.O_CREATE, 0755)
				if err != nil {
					return err
				}

				pieceInfo, err := client.PieceMetaRequest(ctx, routesClient, id)
				if err != nil {
					os.Remove(dataPath)
					return err
				}

				reader, err := client.RetrievePieceRequest(ctx, routesClient, id, 0, pieceInfo.Size)
				if err != nil {
					fmt.Printf("Failed to retrieve file of id: %s\n", id)
					os.Remove(dataPath)
					return err
				}
				defer reader.Close()

				_, err = io.Copy(dataFile, reader)
				if err != nil {
					fmt.Printf("Failed to retrieve file of id: %s\n", id)
					os.Remove(dataPath)
					return err
				}

				fmt.Printf("Successfully retrieved file of id: %s\n", id)
				return nil

			},
		},
		{
			Name:    "delete",
			Aliases: []string{"x"},
			Usage:   "delete data",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return argError.New("Missing data Id")
				}
				err = client.DeletePieceRequest(ctx, routesClient, c.Args().Get(0))

				return err
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
