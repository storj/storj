// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	ctx = context.Background()

	port   string
	apiKey string

	rootCmd = &cobra.Command{Use: "payments"}

	cmdGenerate = &cobra.Command{
		Use:   "generateCSV",
		Short: "generates payment csv",
		Args:  cobra.MinimumNArgs(2),
		RunE:  generateCSV,
	}
)

func main() {
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", ":7778", "satellite port")
	rootCmd.PersistentFlags().StringVarP(&apiKey, "apikey", "a", "abc123", "satellite api key")
	rootCmd.AddCommand(cmdGenerate)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Error", err)
		os.Exit(1)
	}
}

func generateCSV(cmd *cobra.Command, args []string) error {
	//TODO check validity of args

	startTime := args[0]
	endTime := args[1]

	headers := []string{"nodeID", "nodeIDCreationDate", "nodeStatus", "walletAddress", "GBAtRest", "GBBWRepair", "GBBWAudit", "GBBWDownload", "start", "end", "satelliteID"}
	file, err := os.Create("./out/" + startTime + "-" + endTime + ".csv")
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	if err := w.Write(headers); err != nil {
		log.Fatalln("error writing headers to csv:", err)
	}

	qErr := query(startTime, endTime)
	if qErr != nil {
		return err
	}

	if err := w.Error(); err != nil {
		log.Fatal(err)
	}
	w.Flush()

	return query(args[0], args[1])
}

func query(startTime, endTime string) error {
	//maybe return queried result

	return nil
}
