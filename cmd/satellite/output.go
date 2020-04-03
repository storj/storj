// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"io"
	"os"

	"github.com/zeebo/errs"
)

func runWithOutput(output string, fn func(io.Writer) error) (err error) {
	if output == "" {
		return fn(os.Stdout)
	}
	outputTmp := output + ".tmp"
	file, err := os.Create(outputTmp)
	if err != nil {
		return errs.New("unable to create temporary output file: %v", err)
	}
	err = errs.Combine(err, fn(file))
	err = errs.Combine(err, file.Close())
	if err == nil {
		err = errs.Combine(err, os.Rename(outputTmp, output))
	}
	if err != nil {
		return errs.Combine(err, os.Remove(outputTmp))
	}
	return err
}
