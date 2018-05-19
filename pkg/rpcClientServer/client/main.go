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

	"storj.io/storj/pkg/rpcClientServer/client/api"
	"storj.io/storj/pkg/rpcClientServer/client/utils"
	pb "storj.io/storj/pkg/rpcClientServer/protobuf"
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
	client := pb.NewPieceStoreRoutesClient(conn)
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

				fileInfo, err := os.Stat(c.Args().Get(0))

				if fileInfo.IsDir() {
					return argError.New(fmt.Sprintf("Path (%s) is a directory, not a file", c.Args().Get(0)))
				}

				var fileOffset, storeOffset int64 = 0, 0
				var length = fileInfo.Size()
				var ttl = time.Now().Unix() + 86400

				hash, err := utils.DetermineHash(file, fileOffset, length)
				if err != nil {
					return err
				}

				// Created a section reader so that we can concurrently retrieve the same file.
				dataSection := io.NewSectionReader(file, fileOffset, length)

				err = api.StorePieceRequest(ctx, client, hash, dataSection, fileOffset, length, ttl, storeOffset)

				if err != nil {
					fmt.Printf("Failed to store file of hash: %s\n", hash)
					return err
				}

				fmt.Printf("successfully Stored file of hash: %s\n", hash)
				return nil
			},
		},
		{
			Name:    "download",
			Aliases: []string{"d"},
			Usage:   "download data",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return argError.New("No hash specified")
				}

				hash := c.Args().Get(0)

				if c.Args().Get(1) == "" {
					return argError.New("No output file specified")
				}
				_, err := os.Stat(c.Args().Get(1))

				if !os.IsNotExist(err) {
					return argError.New("Path already exists: " + c.Args().Get(1))
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

				pieceInfo, err := api.PieceMetaRequest(ctx, client, hash)
				if err != nil {
					return err
				}

				reader, err := api.RetrievePieceRequest(ctx, client, hash, 0, pieceInfo.Size)
				if err != nil {
					fmt.Printf("Failed to retrieve file of hash: %s\n", hash)
					os.Remove(dataPath)
					return err
				}
				defer reader.Close()

				totalRead := int64(0)
				for totalRead < pieceInfo.Size {
					b := make([]byte, 4096)
					n, err := reader.Read(b)
					if err != nil {
						if err == io.EOF {
							break
						}
						return err
					}

					n, err = dataFile.Write(b[:n])
					if err != nil {
						return err
					}

					totalRead += int64(n)
				}

				if err != nil {
					fmt.Printf("Failed to retrieve file of hash: %s\n", hash)
					os.Remove(dataPath)
					return err
				}

				fmt.Printf("Successfully retrieved file of hash: %s\n", hash)
				return nil

			},
		},
		{
			Name:    "delete",
			Aliases: []string{"x"},
			Usage:   "delete data",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return argError.New("Missing data Hash")
				}
				err = api.DeletePieceRequest(ctx, client, c.Args().Get(0))

				return err
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
