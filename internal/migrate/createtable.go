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

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS table_schemas (id text, schemaText text)`)
	if err != nil {
		return utils.CombineErrors(err, tx.Rollback())
	}

	row := tx.QueryRow(`SELECT (schemaText) FROM table_schemas WHERE id = ?`, identifier)

	var previousSchema string
	err = row.Scan(&previousSchema)

	// not created yet
	if err == sql.ErrNoRows {
		_, err := tx.Exec(schema)
		if err != nil {
			return utils.CombineErrors(err, tx.Rollback())
		}

		_, err = tx.Exec(`INSERT INTO table_schemas(id, schemaText) VALUES (?, ?)`, identifier, schema)
		if err != nil {
			return utils.CombineErrors(err, tx.Rollback())
		}

		return tx.Commit()
	}
	if err != nil {
		return utils.CombineErrors(err, tx.Rollback())
	}

	if schema != previousSchema {
		err := Error.New("schema mismatch:\nold %v\nnew %v", previousSchema, schema)
		return utils.CombineErrors(err, tx.Rollback())
	}

	return tx.Rollback()
}
