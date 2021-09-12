// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil/dbschema"
	"storj.io/private/dbutil/sqliteutil"
	"storj.io/storj/storagenode/storagenodedb"
)

func main() {
	ctx := context.Background()
	log := zap.L()

	err := runSchemaGen(ctx, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
}

func runSchemaGen(ctx context.Context, log *zap.Logger) (err error) {
	storagePath, err := ioutil.TempDir("", "testdb")
	if err != nil {
		return errs.New("Error getting test storage path: %+w", err)
	}
	defer func() {
		removeErr := os.RemoveAll(storagePath)
		if removeErr != nil {
			err = errs.Combine(err, removeErr)
		}
	}()

	db, err := storagenodedb.OpenNew(ctx, log, storagenodedb.Config{
		Storage: storagePath,
		Info:    filepath.Join(storagePath, "piecestore.db"),
		Info2:   filepath.Join(storagePath, "info.db"),
		Pieces:  storagePath,
	})
	if err != nil {
		return errs.New("Error creating new storagenode db: %+w", err)
	}
	defer func() {
		closeErr := db.Close()
		if closeErr != nil {
			err = errs.Combine(err, closeErr)
		}
	}()

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating tables for storagenode db: %+w", err)
	}

	// get schemas
	schemaList := []string{}
	allSchemas := make(map[string]*dbschema.Schema)
	for dbName, dbContainer := range db.SQLDBs {
		schemaList = append(schemaList, dbName)

		nextDB := dbContainer.GetDB()
		schema, err := sqliteutil.QuerySchema(ctx, nextDB)
		if err != nil {
			return errs.New("Error getting schema for db: %+w", err)
		}
		// we don't care about changes in versions table
		schema.DropTable("versions")
		// If tables and indexes of the schema are empty, set to nil
		// to help with comparison to the snapshot.
		if len(schema.Tables) == 0 {
			schema.Tables = nil
		}
		if len(schema.Indexes) == 0 {
			schema.Indexes = nil
		}

		allSchemas[dbName] = schema
	}

	var buf bytes.Buffer

	printf := func(format string, args ...interface{}) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(&buf, format, args...)
	}
	printf(`//lint:file-ignore * generated file
		// AUTOGENERATED BY storj.io/storj/storagenode/storagenodedb/schemagen.go
		// DO NOT EDIT

		package storagenodedb

		import "storj.io/private/dbutil/dbschema"

		func Schema() map[string]*dbschema.Schema {
		return map[string]*dbschema.Schema{
	`)

	// use a consistent order for the generated file
	sort.StringSlice(schemaList).Sort()
	for _, schemaName := range schemaList {
		schema := allSchemas[schemaName]
		(func() {
			printf("%q: &dbschema.Schema{\n", schemaName)
			defer printf("},\n")

			writeErr := WriteSchemaGoStruct(&buf, schema)
			if writeErr != nil {
				err = errs.New("Error writing schema struct: %+w", writeErr)
			}
		})()
		if err != nil {
			return err
		}
	}

	// close bracket for returned map
	printf("}\n")
	// close bracket for Schema() {
	printf("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return errs.New("Error formatting: %+w", err)
	}
	fmt.Println(string(formatted))

	return err
}

func WriteSchemaGoStruct(w io.Writer, schema *dbschema.Schema) (err error) {
	printf := func(format string, args ...interface{}) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, format, args...)
	}

	if len(schema.Tables) > 0 {
		(func() {
			printf("Tables: []*dbschema.Table{\n")
			defer printf("},\n")

			for _, table := range schema.Tables {
				err = WriteTableGoStruct(w, table)
				if err != nil {
					return
				}
				printf(",\n")
			}
		})()
	}

	if len(schema.Indexes) > 0 {
		(func() {
			printf("Indexes: []*dbschema.Index{\n")
			defer printf("},\n")

			for _, index := range schema.Indexes {
				printf("%#v,\n", index)
			}
		})()
	}

	return err
}

func WriteTableGoStruct(w io.Writer, table *dbschema.Table) (err error) {
	printf := func(format string, args ...interface{}) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, format, args...)
	}

	printf("&dbschema.Table{\n")
	defer printf("}")

	printf("Name: %q,\n", table.Name)
	if table.PrimaryKey != nil {
		printf("PrimaryKey: %#v,\n", table.PrimaryKey)
	}
	if table.Unique != nil {
		printf("Unique: %#v,\n", table.Unique)
	}
	if len(table.Columns) > 0 {
		(func() {
			printf("Columns: []*dbschema.Column{\n")
			defer printf("},\n")

			for _, column := range table.Columns {
				err = WriteColumnGoStruct(w, column)
				if err != nil {
					return
				}
			}
		})()
	}

	return err
}

func WriteColumnGoStruct(w io.Writer, column *dbschema.Column) (err error) {
	printf := func(format string, args ...interface{}) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, format, args...)
	}

	printf("&dbschema.Column{\n")
	defer printf("},\n")

	printf("Name: %q,\n", column.Name)
	printf("Type: %q,\n", column.Type)
	printf("IsNullable: %t,\n", column.IsNullable)
	if column.Reference != nil {
		printf("Reference: %#v,\n", column.Reference)
	}

	return err
}
