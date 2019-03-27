// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package pgutil contains utilities for postgres
package pgutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"io"
	"net/url"
	"strconv"
	"strings"
)

const (
	randRetries = 2
)

// RandomStringFromReader creates a random safe string sourced from the specified reader.
func RandomStringFromReader(n int, r io.Reader) string {
	failed := false
	data := make([]byte, n)
	for i := 0; i < randRetries; i++ {
		num, err := r.Read(data)

		// crypto/rand.Read() can fail under different OS versions and OS platforms.
		// We retry the max number of times defined in randRetries.
		if num < n || err != nil {
			failed = true
		} else {
			failed = false
		}
	}

	if failed {
		// If we failed the max number of retries defined in randRetries panic
		// because we can't successfully get a randomized string.
		panic("failed to generate random string")
	}

	return hex.EncodeToString(data)
}

// RandomString creates a random safe string
func RandomString(n int) string {
	return RandomStringFromReader(n, rand.Reader)
}

// ConnstrWithSchema adds schema to a  connection string
func ConnstrWithSchema(connstr, schema string) string {
	schema = strings.ToLower(schema)
	return connstr + "&search_path=" + url.QueryEscape(schema)
}

// QuoteSchema quotes schema name for
func QuoteSchema(schema string) string {
	return strconv.QuoteToASCII(schema)
}

// Execer is for executing sql
type Execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// CreateSchema creates a schema if it doesn't exist.
func CreateSchema(db Execer, schema string) error {
	_, err := db.Exec(`create schema if not exists ` + QuoteSchema(schema) + `;`)
	return err
}

// DropSchema drops the named schema
func DropSchema(db Execer, schema string) error {
	_, err := db.Exec(`drop schema ` + QuoteSchema(schema) + ` cascade;`)
	return err
}
