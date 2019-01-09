// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"log"
	"os"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
)

var (
	ctx     = context.Background()
	rootCmd = &cobra.Command{Use: "payments"}

	cmdGenerate = &cobra.Command{
		Use:   "generateCSV",
		Short: "generates payment csv",
		Args:  cobra.MinimumNArgs(2),
		RunE:  generateCSV,
	}
)

func main() {
	rootCmd.AddCommand(cmdGenerate)
	process.Exec(rootCmd)
}

func generateCSV(cmd *cobra.Command, args []string) error {
	return query(args[0], args[1])
}

func query(startTime, endTime string) error {
	cols := [][]string{
		{"nodeID", "nodeIDCreationDate", "nodeStatus", "walletAddress", "GBAtRest", "GBBWRepair", "GBBWAudit", "GBBWDownload", "start", "end", "satelliteID"},
	}

	file, err := os.Create("./out/" + startTime + "-" + endTime + ".csv")
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)

	for _, record := range cols {
		if err := w.Write(record); err != nil {
			log.Fatalln("error writing record to csv:", err)
		}
	}
	if err := w.Error(); err != nil {
		log.Fatal(err)
	}
	w.Flush()

	return nil
}
