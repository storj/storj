// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package migrate

import (
	"database/sql"
	"strconv"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

// Error is the default migrate errs class
var Error = errs.Class("migrate")

// DB is the minimal implementation that is needed by migration.
type DB interface {
	Begin() (*sql.Tx, error)
	Schema() string
	Rebind(string) string
}

type sqliteDB struct {
	*sql.DB
	schema string
}

// NewSqliteDB creates Sqlite DB wrapper migration purposes
func NewSqliteDB(db *sql.DB, schema string) DB {
	return &sqliteDB{db, schema}
}

// Rebind rebind SQL
func (db *sqliteDB) Rebind(s string) string { return s }

// Schema get schema
func (db *sqliteDB) Schema() string { return db.schema }

type postgresDB struct {
	*sql.DB
	schema string
}

// NewPostgresDB creates Postgres DB wrapper migration purposes
func NewPostgresDB(db *sql.DB, schema string) DB {
	return &postgresDB{db, schema}
}

// Rebind rebind SQL
func (db *postgresDB) Rebind(sql string) string {
	out := make([]byte, 0, len(sql)+10)

	j := 1
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if ch != '?' {
			out = append(out, ch)
			continue
		}

		out = append(out, '$')
		out = append(out, strconv.Itoa(j)...)
		j++
	}

	return string(out)
}

// Schema gets schema
func (db *postgresDB) Schema() string { return db.schema }

// Create with a previous schema check
func Create(identifier string, db DB) error {
	tx, err := db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	schema := db.Schema()

	_, err = tx.Exec(db.Rebind(`CREATE TABLE IF NOT EXISTS table_schemas (id text, schemaText text);`))
	if err != nil {
		return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	row := tx.QueryRow(db.Rebind(`SELECT schemaText FROM table_schemas WHERE id = ?;`), identifier)

	var previousSchema string
	err = row.Scan(&previousSchema)

	// not created yet
	if err == sql.ErrNoRows {
		_, err := tx.Exec(schema)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}

		_, err = tx.Exec(db.Rebind(`INSERT INTO table_schemas(id, schemaText) VALUES (?, ?);`), identifier, schema)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}

		return Error.Wrap(tx.Commit())
	}
	if err != nil {
		return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	if schema != previousSchema {
		err := Error.New("schema mismatch:\nold %v\nnew %v", previousSchema, schema)
		return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	return Error.Wrap(tx.Rollback())
}
