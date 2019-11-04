// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rpcstatus

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// LoggingSanitizer consolidates logging of original errors with sanitization of internal errors.
type LoggingSanitizer struct {
	wrapper         *errs.Class
	log             *zap.Logger
	internalClasses []*errs.Class
}

// NewLoggingSanitizer creates a new LoggingSanitizer.
func NewLoggingSanitizer(wrapper *errs.Class, log *zap.Logger, internalClasses ...*errs.Class) *LoggingSanitizer {
	return &LoggingSanitizer{
		wrapper:         wrapper,
		log:             log,
		internalClasses: internalClasses,
	}
}

// Error logs the message and error to the logger and returns the sanitized error.
func (sanitizer *LoggingSanitizer) Error(msg string, err error) error {
	if sanitizer.log != nil {
		sanitizer.log.Error(msg, zap.Error(sanitizer.wrapper.Wrap(err)))
	}

	err = SanitizeInternalErr(msg, err, sanitizer.internalClasses...)
	if sanitizer.wrapper != nil {
		err = sanitizer.wrapper.Wrap(err)
	}
	return err
}

// SanitizeInternalErr checks if the passed internal error classes has the passed
// error. If so, it returns an rpc-internal error constructed from `msg`; otherwise,
// it returns an rpc-internal wrapped version of the original error.
func SanitizeInternalErr(msg string, err error, internalClasses ...*errs.Class) error {
	for _, class := range internalClasses {
		if class.Has(err) {
			return Error(Internal, errs.New(msg).Error())
		}
	}
	return Error(Internal, err.Error())
}
