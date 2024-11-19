package metagenerator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/xtgo/uuid"
)

func GetPathCount(ctx context.Context, db *sql.DB) (count uint64) {
	// Get path count
	rows, err := db.QueryContext(ctx, `SELECT COUNT(*) FROM objects`)
	if err != nil {
		if err.Error() == `pq: relation "objects" does not exist` {
			return 0
		}
		panic(fmt.Sprintf("failed to get path count: %v", err))
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			panic(err.Error())
		}
	}
	// Check for errors from iterating over rows.
	if err := rows.Err(); err != nil {
		panic(err.Error())
	}
	fmt.Printf("Found %v records\n", count)
	return
}

func GetProjectId(ctx context.Context, db *sql.DB) (projectId uuid.UUID) {
	testFile := &Record{
		Path: "/testFiletoGetPrjectId",
	}
	err := putFile(testFile)
	if err != nil {
		panic(fmt.Sprintf("failed to create test file: %s", err.Error()))
	}

	rows, err := db.QueryContext(ctx, `SELECT project_id FROM objects where object_key = 'testFiletoGetPrjectId'`)
	if err != nil {
		panic(fmt.Sprintf("failed to get project id from db: %s", err.Error()))
	}
	defer rows.Close()

	var data []byte
	for rows.Next() {
		if err := rows.Scan(&data); err != nil {
			panic(err.Error())
		}
	}
	if err := rows.Err(); err != nil {
		panic(err.Error())
	}

	projectId = uuid.FromBytes(data)
	fmt.Printf("Found projectId %s\n", projectId.String())

	err = deleteFile(testFile)
	if err != nil {
		panic(fmt.Sprintf("failed to delete test file: %s", err.Error()))
	}

	return
}
