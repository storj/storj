// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"strings"

	"go.uber.org/zap"
)

var zapNewDevelopment = zap.NewDevelopment
var zapNewProduction = zap.NewProduction
var zapNewNop = zap.NewNop

// NewLogger takes an environment and a set of options for a logger
func NewLogger(e string, options ...zap.Option) (*zap.Logger, error) {
	switch strings.ToLower(e) {
	case "dev", "development":
		return zapNewDevelopment(options...)
	case "prod", "production":
		return zapNewProduction(options...)
	}

	return zapNewNop(), nil
}
