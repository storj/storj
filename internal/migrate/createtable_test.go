package migrate

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"

	_ "github.com/mattn/go-sqlite3"
)

func TestCreateTable(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	assert.NoError(t, err)

	// should create table
	err = CreateTable(db, "example", "CREATE TABLE small (id text)")
	assert.NoError(t, err)

	// shouldn't create a new table
	err = CreateTable(db, "example", "CREATE TABLE small (id text)")
	assert.NoError(t, err)

	// should fail, because schema changed
	err = CreateTable(db, "example", "CREATE TABLE small (id text, version int)")
	assert.Error(t, err)

	// should fail, because of trying to CREATE TABLE with same name
	err = CreateTable(db, "conflict", "CREATE TABLE small (id text, version int)")
	assert.Error(t, err)
}
