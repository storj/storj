package main

import (
	"context"
	"fmt"
	"os"

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

	cmdGetStats.Flags().StringVarP(&port, "port", "p", ":7778", "statdb port")
	cmdGetStats.Flags().StringVarP(&apiKey, "apikey", "a", "abc123", "statdb api key")

	var rootCmd = &cobra.Command{Use: "sdbinspect"}
	rootCmd.AddCommand(cmdGetStats)
	rootCmd.Execute()
}
