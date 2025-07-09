// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

var mon = monkit.Package()

// CreateRandomTestingSchemaName creates a random schema name string.
func CreateRandomTestingSchemaName(n int) string {
	data := make([]byte, n)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
}

// OpenUnique opens a temporary unique CockroachDB database that will be cleaned up when closed.
// It is expected that this should normally be used by way of
// "storj.io/storj/shared/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, connStr string, schemaPrefix string) (db *dbutil.TempDatabase, err error) {
	if !strings.HasPrefix(connStr, "cockroach://") {
		return nil, errs.New("expected a cockroachDB URI, but got %q", connStr)
	}

	schemaName := schemaPrefix + "-" + CreateRandomTestingSchemaName(8)

	masterDB, err := tagsql.Open(ctx, "cockroach", connStr, nil)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, masterDB.Close())
	}()

	err = masterDB.PingContext(ctx)
	if err != nil {
		return nil, errs.New("Could not open masterDB at conn %q: %w", connStr, err)
	}

	_, err = masterDB.ExecContext(ctx, "CREATE DATABASE "+pgutil.QuoteIdentifier(schemaName))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cleanup := func(cleanupDB tagsql.DB) error {
		ctx := context2.WithoutCancellation(ctx)

		// HACKFIX: Set upper time limit for dropping the database.
		// This stall causes flakiness during CI and it's not
		// clear what's the cause.
		//
		// It's better to ignore the DROP than to prevent tests
		// from failing and causing wasted time.
		err := asyncTimeout(ctx, 15*time.Second, func(ctx context.Context) error {
			_, err := cleanupDB.ExecContext(ctx, "DROP DATABASE "+pgutil.QuoteIdentifier(schemaName))
			return err
		})

		// ignore timeout error
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = nil
		}

		return errs.Wrap(err)
	}

	modifiedConnStr, err := changeDBTargetInConnStr(connStr, schemaName)
	if err != nil {
		return nil, errs.Combine(err, cleanup(masterDB))
	}

	sqlDB, err := tagsql.Open(ctx, "cockroach", modifiedConnStr, nil)
	if err != nil {
		return nil, errs.Combine(errs.Wrap(err), cleanup(masterDB))
	}

	dbutil.Configure(ctx, sqlDB, "tmp_cockroach", mon)
	return &dbutil.TempDatabase{
		DB:             sqlDB,
		ConnStr:        modifiedConnStr,
		Schema:         schemaName,
		Driver:         "cockroach",
		Implementation: dbutil.Cockroach,
		Cleanup:        cleanup,
	}, nil
}

func changeDBTargetInConnStr(connStr string, newDBName string) (string, error) {
	connURL, err := url.Parse(connStr)
	if err != nil {
		return "", errs.Wrap(err)
	}
	connURL.Path = newDBName
	return connURL.String(), nil
}

// asyncTimeout starts fn in a goroutine and returns when it doesn't finish in the specified timeout.
func asyncTimeout(parentCtx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	var mu sync.Mutex
	var result error
	var finished bool

	// fn is called inside a goroutine, in case the context
	// hasn't been handled properly.
	go func() {
		defer cancel()
		err := fn(ctx)

		mu.Lock()
		result = err
		finished = true
		mu.Unlock()
	}()

	<-ctx.Done()

	mu.Lock()
	r := result
	if !finished {
		r = ctx.Err()
	}
	mu.Unlock()

	return r
}
