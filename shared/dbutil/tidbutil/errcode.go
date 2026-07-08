// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

const myErrorClassConstraintViolation = "23"

// MySQL server error numbers. See
// https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html.
const (
	myErrBadNullError = 1048 // ER_BAD_NULL_ERROR: column cannot be NULL.
	myErrDupEntry     = 1062 // ER_DUP_ENTRY: duplicate key value.
)

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

// IsDuplicateEntry returns true if the error is a duplicate key violation
// (MySQL errno 1062).
func IsDuplicateEntry(err error) bool {
	var myerr *mysql.MySQLError
	if errors.As(err, &myerr) {
		return myerr.Number == myErrDupEntry
	}
	return false
}

// IsNotNullViolation returns true if the error is a "column cannot be NULL"
// violation (MySQL errno 1048).
func IsNotNullViolation(err error) bool {
	var myerr *mysql.MySQLError
	if errors.As(err, &myerr) {
		return myerr.Number == myErrBadNullError
	}
	return false
}

// IsInvalidSyntax returns whether the query syntax is invalid.
func IsInvalidSyntax(err error) bool {
	code := errorCodeFrom(err)
	return code == "42000" || code == "42601"
}
