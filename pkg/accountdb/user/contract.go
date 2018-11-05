// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package user

import (
	"fmt"
	"storj.io/storj/pkg/accountdb"
)

// Data contract for User entity.
// Describes table and contains all needed stored procedures.
type contract struct {
	accountdb.BaseContract

	// Columns
	colFirstName 	         string
	colLastName 	         string
	colEmail     	         string
	colPassword  	         string

	// Queries
	queryGet				 string
	queryGetByCredentials	 string
	queryInsert 	         string
	queryDelete		         string
	queryUpdate		         string
	queryUpdatePassword		 string
	queryCreateTable         string
}

// Returns query for table creation
func (c *contract) createTableQuery() string {

	return fmt.Sprintf(
		c.queryCreateTable,
		c.TableName(),
		c.Id(),
		c.colFirstName,
		c.colFirstName,
		c.colLastName,
		c.colLastName,
		c.colEmail,
		c.colEmail,
		c.colPassword,
		c.colPassword,
		c.colPassword,
		c.CreationDate(),
		c.colEmail,
	)
}

// Returns query for inserting whole object to db
func (c *contract) insertQuery() string {

	return fmt.Sprintf(
		c.queryInsert,

		c.TableName(),
		c.Id(),
		c.colFirstName,
		c.colLastName,
		c.colEmail,
		c.colPassword,
		c.CreationDate(),
	)
}

// Returns query for selecting user by credentials
func (c *contract) getByCredentialsQuery() string {

	return fmt.Sprintf(
		c.queryGetByCredentials,

		c.TableName(),
		c.colEmail,
		c.colPassword,
	)
}

// Returns query for selecting user by Id
func (c *contract) getQuery() string {

	return fmt.Sprintf(
		c.queryGet,

		c.TableName(),
		c.Id(),
	)
}

// Returns query for deleting user by Id
func (c *contract) deleteQuery() string {

	return fmt.Sprintf(
		c.queryDelete,

		c.TableName(),
		c.Id(),
	)
}

// Returns query for updating user's FirstName, LastName, Email columns
func (c *contract) updateQuery() string {

	return fmt.Sprintf(
		c.queryUpdate,

		c.TableName(),
		c.colFirstName,
		c.colLastName,
		c.colEmail,
		c.Id(),
	)
}

// Returns query for updating user's Password column
func (c *contract) updatePasswordQuery() string {

	return fmt.Sprintf(
		c.queryUpdatePassword,

		c.TableName(),
		c.colPassword,
		c.Id(),
	)
}

// Instance of contract
var userContract = &contract{
	BaseContract: accountdb.NewBaseContract("User"),

	colFirstName: "FirstName",
	colLastName: "LastName",
	colEmail: "Email",
	colPassword: "Password",

	queryGet: `SELECT * FROM %s  
                   WHERE %s = $1`,

	queryGetByCredentials: `SELECT * FROM %s 
                   		        WHERE %s = $1 AND %s = $2`,

	queryInsert: `INSERT INTO %s 
				      (%s, %s, %s, %s, %s, %s)
					     VALUES ($1, $2, $3, $4, $5, datetime('now'))`,

	queryDelete: `DELETE FROM %s 
				      WHERE %s = $1`,

	queryUpdate: `UPDATE %s
                      SET %s = $1, %s = $2, %s = $3
				  	      WHERE %s = $4`,

	queryUpdatePassword: `UPDATE %s
                      	      SET %s = $1
				  	              WHERE %s = $2`,

	queryCreateTable:
					`CREATE TABLE IF NOT EXISTS %s (
					 
					 %s    TEXT        PRIMARY KEY NOT NULL,
					 %s    TEXT        NOT NULL CHECK(%s != ''),
                     %s    TEXT        NOT NULL CHECK(%s != ''),
                     %s    TEXT        NOT NULL CHECK(%s != ''),
					 %s    TEXT        NOT NULL CHECK(%s != '' and length(%s) > 5),
					 %s    DATETIME    NOT NULL,

					 CONSTRAINT Email_Unique UNIQUE (%s));`,
}