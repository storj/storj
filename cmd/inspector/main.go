// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

var (
	// Addr is the address of Capt Planet from command flags
	Addr = flag.String("address", "[::1]:7778", "address of captplanet to inspect")

	// ErrInspectorDial throws when there are errors dialing the inspector server
	ErrInspectorDial = errs.Class("error dialing inspector server:")

	// ErrRequest is for gRPC request errors after dialing
	ErrRequest = errs.Class("error processing request:")

	// ErrIdentity is for errors during identity creation for this CLI
	ErrIdentity = errs.Class("error creating identity:")

	// ErrArgs throws when there are errors with CLI args
	ErrArgs = errs.Class("error with CLI args:")

	// Commander CLI
	rootCmd = &cobra.Command{
		Use:   "inspector",
		Short: "CLI for interacting with Storj Kademlia network",
	}
	kadCmd = &cobra.Command{
		Use:   "kad",
		Short: "commands for kademlia/overlay cache",
	}
	statsCmd = &cobra.Command{
		Use:   "statdb",
		Short: "commands for statdb",
	}
	countNodeCmd = &cobra.Command{
		Use:   "count",
		Short: "count nodes in kademlia and overlay",
		RunE:  CountNodes,
	}
	getBucketsCmd = &cobra.Command{
		Use:   "list-buckets",
		Short: "get all buckets in overlay",
		RunE:  GetBuckets,
	}
	getBucketCmd = &cobra.Command{
		Use:   "ls <bucket_id>",
		Short: "get all nodes in bucket",
		RunE:  GetBucket,
	}
	getStatsCmd = &cobra.Command{
		Use:   "getstats",
		Short: "Get node stats",
		Args:  cobra.MinimumNArgs(1),
		RunE:  GetStats,
	}
	getCSVStatsCmd = &cobra.Command{
		Use:   "getcsvstats",
		Short: "Get node stats from csv",
		Args:  cobra.MinimumNArgs(1),
		RunE:  GetCSVStats,
	}
	createStatsCmd = &cobra.Command{
		Use:   "createstats",
		Short: "Create node with stats",
		Args:  cobra.MinimumNArgs(5), // id, auditct, auditsuccessct, uptimect, uptimesuccessct
		RunE:  CreateStats,
	}
	createCSVStatsCmd = &cobra.Command{
		Use:   "createcsvstats",
		Short: "Create node stats from csv",
		Args:  cobra.MinimumNArgs(1),
		RunE:  CreateCSVStats,
	}
)

// Inspector gives access to kademlia and overlay cache
type Inspector struct {
	identity *provider.FullIdentity
	client   pb.InspectorClient
}

// NewInspector creates a new gRPC inspector server for access to kad
// and the overlay cache
func NewInspector(address string) (*Inspector, error) {
	ctx := context.Background()
	identity, err := node.NewFullIdentity(ctx, 12, 4)
	if err != nil {
		return &Inspector{}, ErrIdentity.Wrap(err)
	}

	tc := transport.NewClient(identity)
	conn, err := tc.DialAddress(ctx, address)
	if err != nil {
		return &Inspector{}, ErrInspectorDial.Wrap(err)
	}

	c := pb.NewInspectorClient(conn)

	return &Inspector{
		identity: identity,
		client:   c,
	}, nil
}

// CountNodes returns the number of nodes in the cache and kademlia
func CountNodes(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	count, err := i.client.CountNodes(context.Background(), &pb.CountNodesRequest{})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("---------- \n - Kademlia: %+v\n - Overlay: %+v\n", count.Kademlia, count.Overlay)
	return nil
}

// GetBuckets returns all buckets in the overlay cache's routing table
func GetBuckets(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	buckets, err := i.client.GetBuckets(context.Background(), &pb.GetBucketsRequest{})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Buckets ---------------- \n Total Buckets: %+v\n", buckets.Total)

	for index, b := range buckets.Ids {
		fmt.Printf("%+v %+v\n", index, b)
	}
	return nil
}

// GetBucket returns a bucket with given `id`
func GetBucket(cmd *cobra.Command, args []string) (err error) {
	if len(args) < 1 {
		return errs.New("Must provide at least one bucket ID")
	}

	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	bucket, err := i.client.GetBucket(context.Background(), &pb.GetBucketRequest{
		Id: args[0],
	})

	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Bucket ----------- \n %+v\n", bucket)
	return nil
}

// GetStats gets a node's stats from statdb
func GetStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	idStr := args[0]

	res, err := i.client.GetStats(context.Background(), &pb.GetStatsRequest{
		NodeId: idStr,
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Stats for ID %s:\n", idStr)
	fmt.Printf("AuditSuccessRatio: %f, UptimeRatio: %f, AuditCount: %d\n",
		res.AuditRatio, res.UptimeRatio, res.AuditCount)
	return nil
}

// GetCSVStats gets node stats from statdb based on a csv
func GetCSVStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	// get csv
	csvPath := args[0]
	csvFile, _ := os.Open(csvPath)
	reader := csv.NewReader(bufio.NewReader(csvFile))
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return ErrArgs.Wrap(err)
		}

		idStr := line[0]
		res, err := i.client.GetStats(context.Background(), &pb.GetStatsRequest{
			NodeId: idStr,
		})
		if err != nil {
			return ErrRequest.Wrap(err)
		}

		fmt.Printf("Stats for ID %s:\n", idStr)
		fmt.Printf("AuditSuccessRatio: %f, UptimeRatio: %f, AuditCount: %d\n",
			res.AuditRatio, res.UptimeRatio, res.AuditCount)
	}
	return nil
}

// CreateStats creates a node with stats in statdb
func CreateStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	idStr := args[0]
	auditCount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return ErrArgs.New("audit count must be an int")
	}
	auditSuccessCount, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return ErrArgs.New("audit success count must be an int")
	}
	uptimeCount, err := strconv.ParseInt(args[3], 10, 64)
	if err != nil {
		return ErrArgs.New("uptime count must be an int")
	}
	uptimeSuccessCount, err := strconv.ParseInt(args[4], 10, 64)
	if err != nil {
		return ErrArgs.New("uptime success count must be an int")
	}

	_, err = i.client.CreateStats(context.Background(), &pb.CreateStatsRequest{
		NodeId:             idStr,
		AuditCount:         auditCount,
		AuditSuccessCount:  auditSuccessCount,
		UptimeCount:        uptimeCount,
		UptimeSuccessCount: uptimeSuccessCount,
	})
	if err != nil {
		return ErrRequest.Wrap(err)
	}

	fmt.Printf("Created statdb entry for ID %s\n", idStr)
	return nil
}

// CreateCSVStats creates node with stats in statdb based on a CSV
func CreateCSVStats(cmd *cobra.Command, args []string) (err error) {
	i, err := NewInspector(*Addr)
	if err != nil {
		return ErrInspectorDial.Wrap(err)
	}

	// get csv
	csvPath := args[0]
	csvFile, _ := os.Open(csvPath)
	reader := csv.NewReader(bufio.NewReader(csvFile))
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return ErrArgs.Wrap(err)
		}

		idStr := line[0]
		auditCount, err := strconv.ParseInt(line[1], 10, 64)
		if err != nil {
			return ErrArgs.New("audit count must be an int")
		}
		auditSuccessCount, err := strconv.ParseInt(line[2], 10, 64)
		if err != nil {
			return ErrArgs.New("audit success count must be an int")
		}
		uptimeCount, err := strconv.ParseInt(line[3], 10, 64)
		if err != nil {
			return ErrArgs.New("uptime count must be an int")
		}
		uptimeSuccessCount, err := strconv.ParseInt(line[4], 10, 64)
		if err != nil {
			return ErrArgs.New("uptime success count must be an int")
		}

		_, err = i.client.CreateStats(context.Background(), &pb.CreateStatsRequest{
			NodeId:             idStr,
			AuditCount:         auditCount,
			AuditSuccessCount:  auditSuccessCount,
			UptimeCount:        uptimeCount,
			UptimeSuccessCount: uptimeSuccessCount,
		})
		if err != nil {
			return ErrRequest.Wrap(err)
		}

		fmt.Printf("Created statdb entry for ID %s\n", idStr)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(kadCmd)
	rootCmd.AddCommand(statsCmd)

	kadCmd.AddCommand(countNodeCmd)
	kadCmd.AddCommand(getBucketsCmd)
	kadCmd.AddCommand(getBucketCmd)

	statsCmd.AddCommand(getStatsCmd)
	statsCmd.AddCommand(getCSVStatsCmd)
	statsCmd.AddCommand(createStatsCmd)
	statsCmd.AddCommand(createCSVStatsCmd)

	flag.Parse()
}

func main() {
	process.Exec(rootCmd)
}
