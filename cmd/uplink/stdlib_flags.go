// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"

	"github.com/zeebo/clingy"
)

type stdlibFlags struct {
	fs *flag.FlagSet
}

func newStdlibFlags(fs *flag.FlagSet) *stdlibFlags {
	return &stdlibFlags{
		fs: fs,
	}
}

func (s *stdlibFlags) Setup(f clingy.Flags) {
	// we use the Transform function to store the value as a side
	// effect so that we can return an error if one occurs through
	// the expected clingy pipeline.
	s.fs.VisitAll(func(fl *flag.Flag) {
		name, _ := flag.UnquoteUsage(fl)
		f.Flag(fl.Name, fl.Usage, fl.DefValue,
			clingy.Advanced,
			clingy.Type(name),
			clingy.Transform(func(val string) (string, error) {
				return "", fl.Value.Set(val)
			}),
		)
	})
}
