// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcerrs

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
)

// StatusMap is used to apply the correct rpc status code to error classes.
type StatusMap map[*errs.Class]rpcstatus.StatusCode

// Log consolidates logging of original errors with sanitization of internal errors.
type Log struct {
	wrapper *errs.Class
	log     *zap.Logger
	codeMap StatusMap
}

// NewLog creates a new Log.
func NewLog(wrapper *errs.Class, log *zap.Logger, codeMap StatusMap) *Log {
	return &Log{
		wrapper: wrapper,
		log:     log,
		codeMap: codeMap,
	}
}

// Error logs the message and error to the logger and returns the mapped rpcstatus code.
func (sanitizer *Log) Error(msg string, err error) error {
	if sanitizer.wrapper != nil {
		err = sanitizer.wrapper.Wrap(err)
	}

	if sanitizer.log != nil {
		sanitizer.log.Error(msg, zap.Error(err))
	}

	for errClass, code := range sanitizer.codeMap {
		if errClass.Has(err) {
			return rpcstatus.Error(code, err.Error())
		}
	}

	if sanitizer.wrapper == nil {
		return rpcstatus.Error(rpcstatus.Internal, msg)
	}
	return rpcstatus.Error(rpcstatus.Internal, sanitizer.wrapper.New("%v", msg).Error())
}
