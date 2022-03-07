// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
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

// transformHumanDate is a helper for command-line flags which accept relative and absolute datetimes.
// It emplements clingy.Option.
var transformHumanDate = clingy.Transform(func(date string) (time.Time, error) {
	switch {
	case date == "none":
		return time.Time{}, nil
	case date == "":
		return time.Time{}, nil
	case date == "now":
		return time.Now(), nil
	case date[0] == '+' || date[0] == '-':
		d, err := time.ParseDuration(date)
		return time.Now().Add(d), errs.Wrap(err)
	default:
		t, err := time.Parse(time.RFC3339, date)
		return t, errs.Wrap(err)
	}
})
