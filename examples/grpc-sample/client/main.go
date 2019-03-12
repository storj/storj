// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/urfave/cli"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
)

// SampleClient allows access to Bidirectional stream method and closing conn
type SampleClient interface {
	Bidirectional(ctx context.Context) error
	CloseConn() error
}

// Client struct has access to pb.SampleClient and grpc conn
type Client struct {
	route pb.SampleClient
	conn  *grpc.ClientConn
}

// NewSampleClient initializes a SampleClient
func NewSampleClient(conn *grpc.ClientConn) SampleClient {
	return &Client{conn: conn, route: pb.NewSampleClient(conn)}
}

// CloseConn closes conn
func (client *Client) CloseConn() error {
	return client.conn.Close()
}

// Bidirectional creates a bidirectional client stream
func (client *Client) Bidirectional(ctx context.Context) error {
	stream, err := client.route.SampleBidirectional(ctx)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		hello := []byte("hello world")
		for i := 0; i < 10000000; i++ {
			fmt.Println(i)
			err := stream.Send(&pb.SampleBidirectionalRequest{
				Data: hello,
			})
			if err != nil {
				// server returns with nil
				if err == io.EOF {
					break
				}
				log.Fatalf("stream Send fail: %s/n", err)
			}
		}
		err := stream.CloseSend()
		if err != nil {
			log.Fatalf("stream send fail: %s\n", err)
		}
		wg.Done()
	}()

	wg.Add(1)

	go func() {
		for {
			res, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Fatalf("stream Recv fail: %s/n", err)
			}
			fmt.Println(string(res.GetData()))
		}
		wg.Done()
	}()
	wg.Wait()

	return nil
}

func main() {
	app := cli.NewApp()

	// Set up connection with rpc server
	var conn *grpc.ClientConn
	conn, err := grpc.Dial(":8080", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("grpc Dial fail: %s/n", err)
	}

	client := NewSampleClient(conn)
	defer client.CloseConn()

	app.Commands = []cli.Command{
		{
			Name:    "bidirectional",
			Aliases: []string{"u"},
			Usage:   "bidirectional data",
			Action: func(c *cli.Context) error {
				err := client.Bidirectional(context.Background())
				if err != nil {
					return err
				}
				return nil
			},
		},
	}
	err = app.Run(os.Args)
	if err != nil {
		log.Fatalf("app Run fail: %s/n", err)
	}
}
