// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	_ "github.com/googleapis/go-sql-spanner" // register the spanner driver
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

var mon = monkit.Package()

var (
	rxDatabaseForm = regexp.MustCompile("^projects/(.*)/instances/(.*)/databases/(.*)$")
	rxInstanceForm = regexp.MustCompile("^projects/(.*)/instances/(.*)$")
)

// CreateRandomTestingDatabaseName creates a random schema name string.
func CreateRandomTestingDatabaseName(n int) string {
	// hex will increase the encoded length by 2 as documented by hex.EncodedLen()
	n /= 2
	data := make([]byte, n)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
}

// OpenUnique opens a spanner database with a temporary unique schema, which will be cleaned up
// when closed. It is expected that this should normally be used by way of
// "storj.io/storj/shared/dbutil/tempdb".OpenUnique() instead of calling it directly.
func OpenUnique(ctx context.Context, connstr string, databasePrefix string) (*dbutil.TempDatabase, error) {
	if !strings.HasPrefix(connstr, "spanner://") {
		return nil, errs.New("expected a spanner URI, but got %q", connstr)
	}

	projectID, instance, _, err := ParseConnStr(connstr)
	if err != nil {
		return nil, errs.New("failed to parse spanner connection string %s: %w", connstr, err)
	}

	// spanner database names are between 2-30 characters
	numRandomCharacters := 4
	possibleRandomCharacters := 30 - len(databasePrefix) - 1
	if numRandomCharacters < possibleRandomCharacters {
		numRandomCharacters = possibleRandomCharacters
	}
	schemaName := databasePrefix + "_" + CreateRandomTestingDatabaseName(numRandomCharacters)
	err = CreateDatabase(ctx, projectID, instance, schemaName)
	if err != nil {
		return nil, errs.New("failed to create database in spanner: %w", err)
	}

	uniqueURL := BuildURL(projectID, instance, &schemaName)
	uniqueDSN := DSNFromURL(uniqueURL)
	db, err := tagsql.Open(ctx, "spanner", uniqueDSN)
	if err == nil {
		// check that connection actually worked before trying createSchema, to make
		// troubleshooting (lots) easier
		err = db.PingContext(ctx)
	}
	if err != nil {
		return nil, errs.New("failed to connect to %q with driver spanner: %w", connstr, err)
	}

	cleanup := func(cleanupDB tagsql.DB) error {
		childCtx, cancel := context2.WithRetimeout(ctx, 15*time.Second)
		defer cancel()
		return dropDatabase(childCtx, uniqueDSN)
	}

	dbutil.Configure(ctx, db, "tmp_spanner", mon)
	return &dbutil.TempDatabase{
		DB:             db,
		ConnStr:        uniqueURL,
		Schema:         schemaName,
		Driver:         "spanner",
		Implementation: dbutil.Spanner,
		Cleanup:        cleanup,
	}, nil
}

// CreateDatabase creates a schema in spanner with the given name.
func CreateDatabase(ctx context.Context, projectID, instanceID, databaseName string) error {
	admin, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database admin: %w", err)
	}

	ddl, err := admin.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          "projects/" + projectID + "/instances/" + instanceID,
		DatabaseDialect: databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL,
		CreateStatement: "CREATE DATABASE " + EscapeIdentifier(databaseName),
		ExtraStatements: []string{},
	})
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	if _, err := ddl.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait database creation: %w", err)
	}
	if err := admin.Close(); err != nil {
		return fmt.Errorf("failed to close database admin: %w", err)
	}
	return nil
}

func dropDatabase(ctx context.Context, dsn string) error {
	admin, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database admin: %w", err)
	}

	if err := admin.DropDatabase(ctx, &databasepb.DropDatabaseRequest{Database: dsn}); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if err := admin.Close(); err != nil {
		return fmt.Errorf("failed to close database admin: %w", err)
	}
	return nil
}

// EscapeIdentifier uses spanner escape characters to escape the given string.
func EscapeIdentifier(s string) string {
	return "`" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "`", "\\`") + "`"
}

// BuildURL takes necessary and optional connection string parameters to build a spanner URL.
func BuildURL(project string, instance string, database *string) string {
	var connStr strings.Builder
	connStr.WriteString("spanner://projects/")
	connStr.WriteString(project)
	connStr.WriteString("/instances/")
	connStr.WriteString(instance)
	if database != nil {
		connStr.WriteString("/databases/")
		connStr.WriteString(*database)
	}
	return connStr.String()
}

// DSNFromURL takes in a Spanner URL and returns back the DSN that is used to connect with Spanner packages.
func DSNFromURL(url string) string {
	return strings.TrimPrefix(url, "spanner://")
}

// ParseConnStr parses a spanner connection string to return the relevant pieces of the connection.
func ParseConnStr(full string) (project, instance string, database *string, err error) {
	trim := strings.TrimPrefix(full, "spanner://")

	matches := rxDatabaseForm.FindStringSubmatch(trim)
	// database is an optional part of a connection string
	if matches == nil || len(matches) != 4 {
		matches = rxInstanceForm.FindStringSubmatch(trim)
		if matches == nil || len(matches) != 3 {
			return "", "", nil, Error.New("database connection should be defined in the form of  'spanner://projects/<PROJECT>/instances/<INSTANCE>/databases/<DATABASE>' or 'spanner://projects/<PROJECT>/instances/<INSTANCE>', but it was %q", full)
		}
	}

	project = matches[1]
	instance = matches[2]
	if len(matches) == 4 {
		database = &matches[3]
	}

	return project, instance, database, nil
}
