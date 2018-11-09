package migrate

import (
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"

	"github.com/lib/pq"
	"github.com/mattn/go-sqlite3"
)

// Error is the default migrate errs class
var Error = errs.Class("migrate")

// CreateTable with a previous schema check
func CreateTable(db *sql.DB, identifier, schema string) error {
	var sqlCreateTable string = `CREATE TABLE IF NOT EXISTS table_schemas (id text, schemaText text);`
	var sqlQueryVersion string
	var sqlInsertVersion string

	switch db.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		sqlQueryVersion = `SELECT schemaText FROM table_schemas WHERE id = ?;`
		sqlInsertVersion = `INSERT INTO table_schemas(id, schemaText) VALUES (?, ?);`
	case *pq.Driver:
		sqlQueryVersion = `SELECT schemaText FROM table_schemas WHERE id = $1;`
		sqlInsertVersion = `INSERT INTO table_schemas(id, schemaText) VALUES ($1, $2);`
	default:
		return errors.New("unknown sql driver")
	}

	tx, err := db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Exec(sqlCreateTable)
	if err != nil {
		return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	row := tx.QueryRow(sqlQueryVersion, identifier)

	var previousSchema string
	err = row.Scan(&previousSchema)

	// not created yet
	if err == sql.ErrNoRows {
		_, err := tx.Exec(schema)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}

		_, err = tx.Exec(sqlInsertVersion, identifier, schema)
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
