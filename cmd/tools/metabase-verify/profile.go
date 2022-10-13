// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"runtime/pprof"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
)

var errProfile = errs.Class("profile")

// IncludeProfiling adds persistent profiling to cmd.
func IncludeProfiling(cmd *cobra.Command) {
	var path string
	var profile *CPUProfile

	flag := cmd.PersistentFlags()
	flag.StringVar(&path, "cpuprofile", "", "write cpu profile to file")

	preRunE := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) (err error) {
		profile, err = NewProfile(path)
		if err != nil {
			return err
		}
		if preRunE != nil {
			return preRunE(cmd, args)
		}
		return nil
	}
	postRunE := cmd.PersistentPostRunE
	cmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) (err error) {
		if postRunE != nil {
			return postRunE(cmd, args)
		}
		profile.Close()
		return nil
	}
}

// CPUProfile contains active profiling information.
type CPUProfile struct{ file *os.File }

// NewProfile starts a new profile on `path`.
func NewProfile(path string) (*CPUProfile, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, errProfile.New("unable to create file: %w", err)
	}

	err = pprof.StartCPUProfile(f)
	return &CPUProfile{file: f}, Error.Wrap(err)
}

// Close finishes the profile.
func (p *CPUProfile) Close() {
	if p == nil || p.file == nil {
		return
	}
	pprof.StopCPUProfile()
}
