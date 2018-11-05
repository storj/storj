package main

import (
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "kad",
		Short: "CLI for interacting with Storj Kademlia network",
	}
)

func main() {
	process.Exec(rootCmd)
}
