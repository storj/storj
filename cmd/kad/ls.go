package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	lsCmd = &cobra.Command{
		Use:   "ls",
		Short: "buckets in node",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Kad ls hit")
		},
	}
)

func init() {
	rootCmd.AddCommand(lsCmd)
}
