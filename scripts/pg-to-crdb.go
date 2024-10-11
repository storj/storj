// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// pg-to-crdb converts a Postgres plaintext sql backup generated by pg_dump
// to a compatible plaintext sql backup that only has SQL statements compatible with CockroachDB.
//
// Usage:
//
//	cat postgres_backup.sql | go run pg-to-crdb.go > cockroach_backup.sql
func main() {
	print := false
	printOnce := false

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(nil, 5242880)
	for scanner.Scan() {
		text := scanner.Text()

		if strings.HasPrefix(text, "CREATE TABLE ") ||
			strings.HasPrefix(text, "CREATE SEQUENCE ") ||
			strings.HasPrefix(text, "COPY ") {
			print = true
		}

		if strings.HasPrefix(text, "ALTER TABLE ") ||
			strings.HasPrefix(text, "CREATE INDEX ") ||
			strings.HasPrefix(text, "GRANT ALL ON TABLE ") ||
			strings.HasPrefix(text, "GRANT ALL ON SEQUENCE ") ||
			strings.HasPrefix(text, "GRANT SELECT ON TABLE ") ||
			strings.HasPrefix(text, "GRANT SELECT ON SEQUENCE ") ||
			strings.HasPrefix(text, "ALTER SEQUENCE ") ||
			(strings.HasPrefix(text, "CREATE UNIQUE INDEX ") && !strings.Contains(text, " WHERE ")) {

			if text[len(text)-1] == ';' {
				printOnce = true
			} else {
				print = true
			}
		}

		if !print && !printOnce {
			continue
		}

		fmt.Println(text)

		if text == ");" || text == "\\." || text == "" {
			print = false
		}
		printOnce = false
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}
