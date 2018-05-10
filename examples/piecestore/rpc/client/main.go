
// Package main implements a simple gRPC client that demonstrates how to use gRPC-Go libraries
// to perform unary, client streaming, server streaming and full duplex RPCs.
//
// It interacts with the route guide service whose definition can be found in routeguide/route_guide.proto.
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"github.com/aleitner/piece-store/rpc-client/api"
	"github.com/aleitner/piece-store/rpc-client/utils"
	"github.com/urfave/cli"
	"github.com/zeebo/errs"
)

var ArgError = errs.Class("argError")

func main() {

	app := cli.NewApp()

	// Set up connection with rpc server
  var conn *grpc.ClientConn
  conn, err := grpc.Dial(":7777", grpc.WithInsecure())
  if err != nil {
    log.Fatalf("did not connect: %s", err)
  }
  defer conn.Close()

	app.Commands = []cli.Command{
    {
      Name:    "upload",
      Aliases: []string{"u"},
      Usage:   "upload data",
      Action:  func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("No input file specified")
				}

				file, err := os.Open(c.Args().Get(0))
				if err != nil {
					return err
				}
				// Close the file when we are done
				defer file.Close()

				fileInfo, err := os.Stat(c.Args().Get(0))

				if fileInfo.IsDir() {
					return ArgError.New(fmt.Sprintf("Path (%s) is a directory, not a file", c.Args().Get(0)))
				}

				var fileOffset, storeOffset int64 = 0, 0
				var length int64 = fileInfo.Size()
				var ttl int64 = time.Now().Unix() + 86400

				hash, err := utils.DetermineHash(file, fileOffset, length)
				if err != nil {
					return err
				}

				// Created a section reader so that we can concurrently retrieve the same file.
				dataSection := io.NewSectionReader(file, fileOffset, length)

				err = api.StoreShardRequest(conn, hash, dataSection, fileOffset, length, ttl, storeOffset)

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
      Action:  func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("No hash specified")
				}

				hash := c.Args().Get(0)

				if c.Args().Get(1) == "" {
					return ArgError.New("No output file specified")
				}
				_, err := os.Stat(c.Args().Get(1))

				if !os.IsNotExist(err) {
					return ArgError.New("Path already exists: "+ c.Args().Get(1))
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

				err = api.RetrieveShardRequest(conn, hash, dataFile, -1, 0)

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
      Action:  func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("Missing data Hash")
				}
				err = api.DeleteShardRequest(conn, c.Args().Get(0))

				return err
      },
    },
  }

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
