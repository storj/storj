// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package pgutil contains utilities for postgres
package pgutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
)

// RandomString creates a random safe string
func RandomString(n int) string {
	data := make([]byte, n)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
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
