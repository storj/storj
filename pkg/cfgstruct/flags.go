// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cfgstruct

import (
	"flag"
	"time"
)

// FlagSet is an interface that matches both *flag.FlagSet and *pflag.FlagSet
type FlagSet interface {
	BoolVar(p *bool, name string, value bool, usage string)
	IntVar(p *int, name string, value int, usage string)
	Int64Var(p *int64, name string, value int64, usage string)
	UintVar(p *uint, name string, value uint, usage string)
	Uint64Var(p *uint64, name string, value uint64, usage string)
	DurationVar(p *time.Duration, name string, value time.Duration, usage string)
	Float64Var(p *float64, name string, value float64, usage string)
	StringVar(p *string, name string, value string, usage string)
}

var _ FlagSet = (*flag.FlagSet)(nil)
