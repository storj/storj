// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	files := []string{}
	globPaths, err := filepath.Glob("*.dbx")
	if err != nil {
		log.Fatal(err)
	}
	sort.Strings(globPaths) // sort so files are always in a specific order regardless of os
	for _, p := range globPaths {
		if p != "satellitedb.dbx" {
			files = append(files, fmt.Sprintf("-i=%s", p))
		}
	}
	files = append(files, "satellitedb.dbx", ".")

	// final commands look like `dbx schema -d pgx -d pgxcockroach -i accounting.dbx ... -i user.dbx satellitedb.dbx .`
	// and `dbx golang -d pgx -d pgxcockroach -t templates -i accounting.dbx ... -i user.dbx satellitedb.dbx .`
	schemaArgs := append([]string{"schema", "-d=pgx", "-d=pgxcockroach"}, files...)
	schemaOut, err := exec.Command("dbx", schemaArgs...).CombinedOutput()
	if err != nil {
		fmt.Println("schema out", string(schemaOut))
		log.Fatal(err)
	}
	gogenArgs := append([]string{"golang", "-d=pgx", "-d=pgxcockroach", "-p=dbx", "-t=templates"}, files...)
	gogenOut, err := exec.Command("dbx", gogenArgs...).CombinedOutput()
	if err != nil {
		fmt.Println("gogen out", string(gogenOut))
		log.Fatal(err)
	}

	originalDBXBytes, err := os.ReadFile("satellitedb.dbx.go")
	if err != nil {
		log.Fatal(err)
	}
	replacer := strings.NewReplacer(
		"*sql.Tx", "tagsql.Tx",
		"*sql.Rows", "tagsql.Rows",
		`_ "github.com/jackc/pgx/v4/stdlib"`, `"storj.io/private/tagsql"`,
		`_ "github.com/jackc/pgx/v5/stdlib"`, `"storj.io/private/tagsql"`,
		`"github.com/jackc/pgx/v5/pgconn"`, `"github.com/jackc/pgconn"`,
		"type DB struct {\n\t*sql.DB", "type DB struct {\n\ttagsql.DB",
		"db = &DB{\n\t\tDB: sql_db", "db = &DB{\n\t\tDB: tagsql.Wrap(sql_db)",
	)
	newDBX := replacer.Replace(string(originalDBXBytes))
	fileString := "//lint:file-ignore U1000,ST1012 generated file\n" + newDBX

	err = os.WriteFile("satellitedb.dbx.go", []byte(fileString), 0755)
	if err != nil {
		log.Fatal(err)
	}
}
