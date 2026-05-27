// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package tidbutil contains utilities for TiDB.
package tidbutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

var mon = monkit.Package()

// QuoteIdentifier quotes a TiDB/MySQL identifier with backticks, escaping any
// embedded backticks.
func QuoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// CreateRandomTestingSchemaName returns a hex-encoded random string of length 2*n.
func CreateRandomTestingSchemaName(n int) string {
	data := make([]byte, n)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}
	return hex.EncodeToString(data)
}

// OpenUnique opens a TiDB database scoped to a temporary, uniquely named MySQL
// database which is dropped when the returned *dbutil.TempDatabase is closed.
// connURL must use the tidb:// scheme.
func OpenUnique(ctx context.Context, connURL string, namePrefix string) (_ *dbutil.TempDatabase, err error) {
	if !strings.HasPrefix(connURL, "tidb://") {
		return nil, errs.New("expected a tidb:// URL, got %q", connURL)
	}

	schemaName := sanitizeIdentifier(namePrefix, 64-1-16) + "_" + CreateRandomTestingSchemaName(8)

	masterURL, err := withDatabasePath(connURL, "")
	if err != nil {
		return nil, errs.Wrap(err)
	}

	masterDB, err := tagsql.Open(ctx, DriverName, masterURL, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, masterDB.Close()) }()

	if err := masterDB.PingContext(ctx); err != nil {
		return nil, errs.New("could not connect to TiDB at %q: %w", masterURL, err)
	}

	if _, err := masterDB.ExecContext(ctx, "CREATE DATABASE "+QuoteIdentifier(schemaName)); err != nil {
		return nil, errs.Wrap(err)
	}

	cleanup := func(d tagsql.DB) error {
		cctx, cancel := context.WithTimeout(context2.WithoutCancellation(ctx), 15*time.Second)
		defer cancel()
		_, err := d.ExecContext(cctx, "DROP DATABASE "+QuoteIdentifier(schemaName))
		return errs.Wrap(err)
	}

	targetURL, err := withDatabasePath(connURL, schemaName)
	if err != nil {
		return nil, errs.Combine(errs.Wrap(err), cleanup(masterDB))
	}

	sqlDB, err := tagsql.Open(ctx, DriverName, targetURL, nil)
	if err != nil {
		return nil, errs.Combine(errs.Wrap(err), cleanup(masterDB))
	}

	dbutil.Configure(ctx, sqlDB, "tmp_tidb", mon)
	return &dbutil.TempDatabase{
		DB:             sqlDB,
		ConnStr:        targetURL,
		Schema:         schemaName,
		Driver:         DriverName,
		Implementation: dbutil.TiDB,
		Cleanup:        cleanup,
	}, nil
}

func withDatabasePath(connURL, dbName string) (string, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return "", fmt.Errorf("invalid tidb URL: %w", err)
	}
	if dbName == "" {
		u.Path = ""
	} else {
		u.Path = "/" + dbName
	}
	return u.String(), nil
}

// sanitizeIdentifier trims a candidate identifier so the eventual MySQL database name
// stays under MySQL's 64-character limit and contains only safe characters.
func sanitizeIdentifier(name string, maxLength int) string {
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	if len(sanitized) > maxLength {
		return sanitized[:maxLength]
	}
	return sanitized
}
