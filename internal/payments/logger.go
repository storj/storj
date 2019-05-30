// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"fmt"

	"github.com/stripe/stripe-go"
	"go.uber.org/zap"
)

// zapWrapper is a wrapper for *zap.Logger.
// Implements stripe.LeveledLoggerInterface
type zapWrapper struct {
	log *zap.Logger
}

func (w *zapWrapper) Debugf(format string, v ...interface{}) {
	w.log.Debug(fmt.Sprintf(format, v...))
}

func (w *zapWrapper) Errorf(format string, v ...interface{}) {
	w.log.Error(fmt.Sprintf(format, v...))
}

func (w *zapWrapper) Infof(format string, v ...interface{}) {
	w.log.Info(fmt.Sprintf(format, v...))
}

func (w *zapWrapper) Warnf(format string, v ...interface{}) {
	w.log.Warn(fmt.Sprintf(format, v...))
}

// wrapLogger wraps *zap.Logger into stripe.LeveledLoggerInterface
func wrapLogger(log *zap.Logger) stripe.LeveledLoggerInterface {
	return &zapWrapper{log: log}
}
