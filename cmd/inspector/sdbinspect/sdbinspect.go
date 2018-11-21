// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package sdbinspect

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/provider"
	pb "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/statdb/sdbclient"
)

func main() {
	ctx := context.Background()
	var port string
	var apiKey string

	var cmdGetStats = &cobra.Command{
		// get stats for specific node
		Use:   "get",
		Short: "Print node stats",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := getSdbClient(ctx, port, apiKey)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

			nodeID := node.IDFromString(args[0])
			nodeStats, err := client.Get(ctx, nodeID.Bytes())
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

			fmt.Printf("Stats for ID %s:\n", nodeID.String())
			fmt.Printf("AuditSuccessRatio: %f, UptimeRatio: %f, AuditCount: %d\n",
				nodeStats.AuditSuccessRatio, nodeStats.UptimeRatio, nodeStats.AuditCount)
		},
	}

	var cmdGetCSVStats = &cobra.Command{
		// get node stats from csv
		Use:   "getcsv",
		Short: "Print node stats from csv",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := getSdbClient(ctx, port, apiKey)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
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
					fmt.Println("Error", err)
					os.Exit(1)
				}

				idStr := line[0]
				nodeID := node.IDFromString(idStr)

				nodeStats, err := client.Get(ctx, nodeID.Bytes())
				if err != nil {
					fmt.Println("Error", err)
					os.Exit(1)
				}

				fmt.Printf("Stats for ID %s:\n", nodeID.String())
				fmt.Printf("AuditSuccessRatio: %f, UptimeRatio: %f, AuditCount: %d\n",
					nodeStats.AuditSuccessRatio, nodeStats.UptimeRatio, nodeStats.AuditCount)
			}
		},
	}

	var cmdCreateStats = &cobra.Command{
		// create node stats from csv
		Use:   "create",
		Short: "Create node with stats",
		Args:  cobra.MinimumNArgs(5), // id, auditct, auditsuccessct, uptimect, uptimesuccessct
		Run: func(cmd *cobra.Command, args []string) {
			client, err := getSdbClient(ctx, port, apiKey)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

			idStr := args[0]
			nodeID := node.IDFromString(idStr)

			auditCount, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			auditSuccessCount, err := strconv.ParseInt(args[2], 10, 64)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			uptimeCount, err := strconv.ParseInt(args[3], 10, 64)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			uptimeSuccessCount, err := strconv.ParseInt(args[4], 10, 64)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

			stats := &pb.NodeStats{
				AuditCount:         auditCount,
				AuditSuccessCount:  auditSuccessCount,
				UptimeCount:        uptimeCount,
				UptimeSuccessCount: uptimeSuccessCount,
			}
			err = client.CreateWithStats(ctx, nodeID.Bytes(), stats)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

			fmt.Printf("Created statdb entry for ID %s\n", nodeID.String())
		},
	}

	var cmdCreateCSVStats = &cobra.Command{
		// create node stats from csv
		Use:   "createcsv",
		Short: "Create node stats from csv",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := getSdbClient(ctx, port, apiKey)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
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
					fmt.Println("Error", err)
					os.Exit(1)
				}

				idStr := line[0]
				nodeID := node.IDFromString(idStr)

				auditCount, err := strconv.ParseInt(line[1], 10, 64)
				if err != nil {
					fmt.Println("Error", err)
					os.Exit(1)
				}
				auditSuccessCount, err := strconv.ParseInt(line[2], 10, 64)
				if err != nil {
					fmt.Println("Error", err)
					os.Exit(1)
				}
				uptimeCount, err := strconv.ParseInt(line[3], 10, 64)
				if err != nil {
					fmt.Println("Error", err)
					os.Exit(1)
				}
				uptimeSuccessCount, err := strconv.ParseInt(line[4], 10, 64)
				if err != nil {
					fmt.Println("Error", err)
					os.Exit(1)
				}

				stats := &pb.NodeStats{
					AuditCount:         auditCount,
					AuditSuccessCount:  auditSuccessCount,
					UptimeCount:        uptimeCount,
					UptimeSuccessCount: uptimeSuccessCount,
				}
				err = client.CreateWithStats(ctx, nodeID.Bytes(), stats)
				if err != nil {
					fmt.Println("Error", err)
					os.Exit(1)
				}

				fmt.Printf("Created statdb entry for ID %s\n", nodeID.String())
			}
		},
	}

	cmdGetStats.Flags().StringVarP(&port, "port", "p", ":7778", "statdb port")
	cmdGetStats.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "statdb api key")
	cmdGetCSVStats.Flags().StringVarP(&port, "port", "p", ":7778", "statdb port")
	cmdGetCSVStats.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "statdb api key")
	cmdCreateStats.Flags().StringVarP(&port, "port", "p", ":7778", "statdb port")
	cmdCreateStats.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "statdb api key")
	cmdCreateCSVStats.Flags().StringVarP(&port, "port", "p", ":7778", "statdb port")
	cmdCreateCSVStats.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "statdb api key")

	var rootCmd = &cobra.Command{Use: "sdbinspect"}
	rootCmd.AddCommand(cmdGetStats, cmdGetCSVStats, cmdCreateStats, cmdCreateCSVStats)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
}

func getSdbClient(ctx context.Context, port, apiKey string) (sdbclient.Client, error) {
	ca, err := provider.NewTestCA(ctx)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	client, err := sdbclient.NewClient(identity, port, []byte(apiKey))
	if err != nil {
		return nil, err
	}

	return client, nil
}
