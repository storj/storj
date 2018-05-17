// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"strings"

	"go.uber.org/zap"
)

// NewLogger takes an environment and a set of options for a logger
func NewLogger(e string, options ...zap.Option) (*zap.Logger, error) {
	switch strings.ToLower(e) {
	case "dev", "development":
		return zap.NewDevelopment(options...)
	case "prod", "production":
		return zap.NewProduction(options...)
	}

	return zap.NewNop(), nil
}
