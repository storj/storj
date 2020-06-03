// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"database/sql"
	"reflect"
	"runtime"

	"storj.io/private/version"
)

func leakCheckRows(rows *sql.Rows) *sql.Rows {
	if !version.Build.Release && rows != nil {
		runtime.SetFinalizer(rows, ensureRowsClosed)
	}
	return rows
}

func ensureRowsClosed(rows *sql.Rows) {
	// this field is protected by a mutex, but fortunately for us, we don't
	// have to worry because we know we are the only ones with a reference
	// to this object since it's running in the finalizer. race free!
	if !reflect.ValueOf(rows).Elem().FieldByName("closed").Bool() {
		panic("leaked *sql.Rows value without being closed")
	}
}

func leakCheckTx(tx *sql.Tx) *sql.Tx {
	if !version.Build.Release && tx != nil {
		runtime.SetFinalizer(tx, ensureTxComplete)
	}
	return tx
}

func ensureTxComplete(tx *sql.Tx) {
	// from the docs of the struct:
	//
	//     done transitions from 0 to 1 exactly once, on Commit
	//     or Rollback. once done, all operations fail with
	//     ErrTxDone.
	//     Use atomic operations on value when checking value.
	//
	// fortunately, we're the only reference to this tx, so we don't
	// have to worry about atomics.

	if reflect.ValueOf(tx).Elem().FieldByName("done").Int() != 1 {
		panic("leaked *sql.Tx value without being complete")
	}
}

func leakCheckConn(conn *sql.Conn) *sql.Conn {
	if !version.Build.Release && conn != nil {
		runtime.SetFinalizer(conn, ensureConnComplete)
	}
	return conn
}

func ensureConnComplete(conn *sql.Conn) {
	// from the docs of the struct:
	//
	//     done transitions from 0 to 1 exactly once, on close.
	//     Once done, all operations fail with ErrConnDone.
	//     Use atomic operations on value when checking value.
	//
	// fortunately, we're the only reference to this tx, so we don't
	// have to worry about atomics.

	if reflect.ValueOf(conn).Elem().FieldByName("done").Int() != 1 {
		panic("leaked *sql.Conn value without being complete")
	}
}

func leakCheckStmt(stmt *sql.Stmt) *sql.Stmt {
	if !version.Build.Release && stmt != nil {
		runtime.SetFinalizer(stmt, ensureStmtClosed)
	}
	return stmt
}

func ensureStmtClosed(stmt *sql.Stmt) {
	// this field is protected by a mutex, but fortunately for us, we don't
	// have to worry because we know we are the only ones with a reference
	// to this object since it's running in the finalizer. race free!
	if !reflect.ValueOf(stmt).Elem().FieldByName("closed").Bool() {
		panic("leaked *sql.Stmt value without being closed")
	}
}

func leakCheckRow(row *sql.Row) *sql.Row {
	if !version.Build.Release && row != nil {
		runtime.SetFinalizer(row, ensureRowClosed)
	}
	return row
}

func ensureRowClosed(row *sql.Row) {
	// check the underlying rows field, avoiding issue if it is nil.
	rows := reflect.ValueOf(row).Elem().FieldByName("rows")
	if rows.IsNil() {
		return
	}

	// this field is protected by a mutex, but fortunately for us, we don't
	// have to worry because we know we are the only ones with a reference
	// to this object since it's running in the finalizer. race free!
	if !rows.Elem().FieldByName("closed").Bool() {
		panic("leaked *sql.Rows value without being closed")
	}
}
