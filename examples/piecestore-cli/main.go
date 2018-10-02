// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	pstore "storj.io/storj/pkg/piecestore"
	"storj.io/storj/pkg/process"
)

var argError = errs.Class("argError")

func storeMain(cmd *cobra.Command, args []string) error {
	id := args[0]
	path := args[1]
	outputDir := args[2]

	file, err := os.Open(path)
	if err != nil {
		return err
	}

	// Close the file when we are done
	defer printError(file.Close)

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		return argError.New(fmt.Sprintf("Path (%s) is a directory, not a file", path))
	}

	dataFileChunk, err := pstore.StoreWriter(id, outputDir)
	if err != nil {
		return err
	}

	// Close when finished
	defer printError(dataFileChunk.Close)

	_, err = io.Copy(dataFileChunk, file)

	return err
}

func retrieveMain(cmd *cobra.Command, args []string) error {
	id := args[0]
	path := args[1]

	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !fileInfo.IsDir() {
		return argError.New(fmt.Sprintf("Path (%s) is a file, not a directory", path))
	}

	dataFileChunk, err := pstore.RetrieveReader(context.Background(), id, 0, -1, path)
	if err != nil {
		return err
	}

	// Close when finished
	defer printError(dataFileChunk.Close)

	_, err = io.Copy(os.Stdout, dataFileChunk)
	return err
}

func deleteMain(cmd *cobra.Command, args []string) error {
	id := args[0]
	directory := args[1]
	return pstore.Delete(id, directory)
}

func main() {
	root := &cobra.Command{
		Use:   "piecestore-cli",
		Short: "piecestore example cli",
	}

	root.AddCommand(
		&cobra.Command{
			Use:       "store [id] [dataPath] [storeDir]",
			Aliases:   []string{"s"},
			Short:     "Store data by id",
			Args:      cobra.ExactArgs(3),
			ValidArgs: []string{"id", "datapath", "storedir"},
			RunE:      storeMain,
		},
		&cobra.Command{
			Use:     "retrieve [id] [storeDir]",
			Aliases: []string{"r"},
			Args:    cobra.ExactArgs(2),
			Short:   "Retrieve data by id and print to Stdout",
			RunE:    retrieveMain,
		},
		&cobra.Command{
			Use:     "delete [id] [storeDir]",
			Aliases: []string{"d"},
			Args:    cobra.ExactArgs(2),
			Short:   "Delete data by id",
			RunE:    deleteMain,
		},
	)

	process.Exec(root)
}

func printError(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}
