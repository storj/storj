package main

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
			nodeID := node.IDFromString(args[0])

			ca, err := provider.NewTestCA(ctx)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			identity, err := ca.NewIdentity()
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			client, err := sdbclient.NewClient(identity, port, []byte(apiKey))
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}

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

	var cmdCreateCSVStats = &cobra.Command{
		// create node stats from csv
		Use:   "createcsv",
		Short: "Print node stats",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ca, err := provider.NewTestCA(ctx)
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			identity, err := ca.NewIdentity()
			if err != nil {
				fmt.Println("Error", err)
				os.Exit(1)
			}
			client, err := sdbclient.NewClient(identity, port, []byte(apiKey))
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

				err = client.CreateWithStats(ctx, nodeID.Bytes(), auditCount, auditSuccessCount, uptimeCount, uptimeSuccessCount)
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
	cmdCreateCSVStats.Flags().StringVarP(&port, "port", "p", ":7778", "statdb port")
	cmdCreateCSVStats.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "statdb api key")

	var rootCmd = &cobra.Command{Use: "sdbinspect"}
	rootCmd.AddCommand(cmdGetStats, cmdCreateCSVStats)
	rootCmd.Execute()
}
