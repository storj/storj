// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const myErrorClassConstraintViolation = "23"

// errorCodeFrom returns the SQLSTATE error code from the given error, if any.
func errorCodeFrom(err error) string {
	var myerr *mysql.MySQLError
	if errors.As(err, &myerr) {
		return string(myerr.SQLState[:])
	}
	return ""
}

// IsConstraintViolation returns true if provided error belongs to Integrity
// Constraint Violation, Class 23.
func IsConstraintViolation(err error) bool {
	return strings.HasPrefix(errorCodeFrom(err), myErrorClassConstraintViolation)
}

// IsInvalidSyntax returns whether the query syntax is invalid.
func IsInvalidSyntax(err error) bool {
	code := errorCodeFrom(err)
	return code == "42000" || code == "42601"
}
