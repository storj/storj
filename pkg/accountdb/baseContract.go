// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accountdb

// Base data contract for all entities.
// Describes all common columns and stored procedures.
type BaseContract interface {
	TableName() string

	// common columns
	Id() string
	CreationDate() string

	// common queries
	CreateTableQuery() string
}

type baseContract struct {
	tableName string
	colId  string
	colCreationDate string
}


func NewBaseContract(tableName string) *baseContract {
	return &baseContract{
		tableName: tableName,
		colId: "Id",
		colCreationDate: "CreationDate",
	}
}

func (b *baseContract) Id() string {
	return b.colId
}

func (b *baseContract) CreationDate() string {
	return b.colCreationDate
}

func (b *baseContract) TableName() string {
	return b.tableName
}

func (b *baseContract) CreateTableQuery() string {
	return ""
}