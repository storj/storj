// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build windows
// +build ignore

package main

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
)

const exitStr = "EXIT"

var (
	parseCmd = &cobra.Command{
		Use:  "parse <scheduled task output path>",
		Args: cobra.ExactArgs(1),
		RunE: cmdParse,
	}

	exitBytes = []byte(exitStr)
)

func cmdParse(cmd *cobra.Command, args []string) error {
	file, err := os.Open(args[0])
	if err != nil {
		return errs.Wrap(err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(scanExit)
	for scanner.Scan() {
		pairs := strings.Split(scanner.Text(), exitStr)
		for i, line := range pairs {
			line = strings.Trim(line, " \t\n")
			if (i % 2) == 0 {
				log.Println(line)
			} else {
				if line != "0" {
					return errs.New("non-zero exit: %s", line)
				}
			}
		}
	}
	return nil
}

func scanExit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, exitBytes); i >= 0 {
		if j := bytes.IndexByte(data[i:len(data)], '\n'); j >= 0 {
			trimmed := bytes.Trim(data[0:i+j], " \t\n")
			return i + j + 1, trimmed, nil
		}
	}

	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func main() {
	err := parseCmd.Execute()
	if err != nil {
		log.Fatal(errs.Wrap(err))
	}
}
