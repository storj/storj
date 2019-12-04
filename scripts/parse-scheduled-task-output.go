// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build windows
// +build ignore

package main

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
)

const exitStr = "EXIT"

var (
	parseCmd = &cobra.Command{
		Use:  "parse <task name> <scheduled task output path>",
		Args: cobra.ExactArgs(2),
		RunE: cmdParse,
	}

	exitBytes = []byte(exitStr)
)

func cmdParse(cmd *cobra.Command, args []string) error {
	if err := waitForTaskReady(args[0]); err != nil {
		return errs.Wrap(err)
	}

	file, err := os.Open(args[1])
	if err != nil {
		return errs.Wrap(err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(scanExit)
	for scanner.Scan() {
		pairs := strings.Split(scanner.Text(), exitStr)
		line := strings.Trim(pairs[0], " \t\n")
		if line != "" {
			log.Println(line)
		}
		if len(pairs) != 2 {
			return errs.New("no exit status logged")
		}
		if pairs[1] != "0" {
			return errs.New("non-zero exit: %s", line)
		}
	}
	return nil
}

func waitForTaskReady(taskName string) error {
	args := []string{
		"/c", "schtasks", "/query", "/fo", "list", "/tn", taskName,
	}
	// TODO: maybe move duration to a flag and/or receive ctx
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	var notReady = false
	for {
		if err := ctx.Err(); err != nil {
			if err == context.DeadlineExceeded {
				return nil
			}
			return err
		}

		var status string
		out, err := exec.Command("cmd", args...).CombinedOutput()
		if err != nil {
			return errs.New(string(out))
		}

		scanner := bufio.NewScanner(bytes.NewBuffer(out))
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.Trim(line, " \t\n")

			if strings.HasPrefix(line, "Status:") {
				pair := strings.Split(line, ":")
				status = strings.Trim(pair[1], " \t\n")
				break
			}
		}

		switch status {
		case "":
			return errs.New("no task status found in output:\n%s", out)
		case "Ready":
			if notReady {
				cancel()
				return nil
			}
		default:
			if !notReady {
				notReady = true
			}
		}

		time.Sleep(250 * time.Millisecond)
	}
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
		// TODO: remove unwanted usage info
		log.Fatal(errs.Wrap(err))
	}
}
