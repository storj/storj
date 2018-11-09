package migrate

import (
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

var Error = errs.Class("migrate")

// CreateTable with a previous schema check
func CreateTable(db *sql.DB, identifier, schema string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS table_schemas (id text, schemaText text);`)
	if err != nil {
		return utils.CombineErrors(err, tx.Rollback())
	}

	row := tx.QueryRow(`SELECT (schemaText) FROM table_schemas WHERE id = ?`, identifier)
	// not created yet
	if row == nil {
		_, err := tx.Exec(schema)
		if err != nil {
			return utils.CombineErrors(err, tx.Rollback())
		}

		_, err = tx.Exec(`INSERT table_schemas(id, schemaText) VALUES (?, ?)`, identifier, schema)
		if err != nil {
			return utils.CombineErrors(err, tx.Rollback())
		}

		return tx.Commit()
	}

	var previousSchema string
	err = row.Scan(&previousSchema)
	if err != nil {
		return utils.CombineErrors(err, tx.Rollback())
	}

	if schema != previousSchema {
		err := Error.New("schema mismatch:\nold %v\nnew %v", previousSchema, newSchema)
		return utils.CombineErrors(err, tx.Rollback())
	}

	return tx.Rollback()
}
