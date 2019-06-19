// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cfgstruct

import (
	"time"

	"github.com/spf13/pflag"
)

// FlagSet is an interface that matches *pflag.FlagSet
type FlagSet interface {
	BoolVar(p *bool, name string, value bool, usage string)
	IntVar(p *int, name string, value int, usage string)
	Int64Var(p *int64, name string, value int64, usage string)
	UintVar(p *uint, name string, value uint, usage string)
	Uint64Var(p *uint64, name string, value uint64, usage string)
	DurationVar(p *time.Duration, name string, value time.Duration, usage string)
	Float64Var(p *float64, name string, value float64, usage string)
	StringVar(p *string, name string, value string, usage string)
	StringArrayVar(p *[]string, name string, value []string, usage string)
	Var(val pflag.Value, name string, usage string)
	MarkHidden(name string) error
}

var _ FlagSet = (*pflag.FlagSet)(nil)
