package migrate

import (
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

// Error is the default migrate errs class
var Error = errs.Class("migrate")

// CreateTable with a previous schema check
func CreateTable(db *sql.DB, identifier, schema string) error {
	tx, err := db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS table_schemas (id text, schemaText text)`)
	if err != nil {
		return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
	}

	row := tx.QueryRow(`SELECT (schemaText) FROM table_schemas WHERE id = ?`, identifier)

	var previousSchema string
	err = row.Scan(&previousSchema)

	// not created yet
	if err == sql.ErrNoRows {
		_, err := tx.Exec(schema)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}

		_, err = tx.Exec(`INSERT INTO table_schemas(id, schemaText) VALUES (?, ?)`, identifier, schema)
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
