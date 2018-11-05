// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package user

import (
	"database/sql"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"storj.io/storj/pkg/accountdb"
	"time"
)

// Exposes methods to manage User table in database.
type Repository struct {
	accountdb.BaseRepository
	*contract
}

// Constructor for user repository
func NewRepository() *Repository {
	return &Repository {
		accountdb.NewBaseRepo(userContract),
		userContract,
	}
}

// Method for querying user by credentials from the database.
func (r *Repository) GetByCredentials(password, email string) (*User, error) {

	res, err := r.Query(func(conn *sql.DB) (interface{}, error) {

		rows, err := conn.Query(r.getByCredentialsQuery(), email, password)

		defer rows.Close()

		if err != nil {
			return nil, nil
		}

		for rows.Next(){

			var id uuid.UUID
			var name, lastName, pass, email string
			var date time.Time

			err := rows.Scan(&id, &name, &lastName, &email, &pass, &date)
			if err != nil{
				return nil, err
			}

			return NewUser(id, name, lastName, email, pass, date), nil
		}

		return nil, errors.New("No data found")
	})

	return res.(*User), err
}

// Method for querying user by id from the database.
func (r *Repository) Get(id uuid.UUID) (*User, error) {

	res, err := r.Query(func(conn *sql.DB) (interface{}, error) {

		rows, err := conn.Query(r.getQuery(), id)

		defer rows.Close()

		if err != nil {
			return nil, nil
		}

		for rows.Next(){

			var id uuid.UUID
			var name, lastName, pass, email string
			var date time.Time

			err := rows.Scan(&id, &name, &lastName, &email, &pass, &date)
			if err != nil{
				return nil, err
			}

			return NewUser(id, name, lastName, email, pass, date), nil
		}

		return nil, errors.New("No data found")
	})

	if res == nil {
		return nil, errors.New("No data found")
	}

	return res.(*User), err
}

// Method for inserting user into the database
func (r *Repository) Insert(user *User) (error) {

	return r.Exec(func(conn *sql.DB) (error) {

		_, err := conn.Exec(r.insertQuery(),
							user.Id(),
							user.firstName,
							user.lastName,
							user.email,
							user.password)

		if err != nil {
			return err
		}

		return nil
	})
}

// Method for deleting user by Id from the database.
func (r *Repository) Delete(id uuid.UUID) (error) {

	return r.Exec(func(conn *sql.DB) (error) {

		_, err := conn.Exec(r.deleteQuery(), id)

		if err != nil {
			return err
		}

		return nil
	})
}

// Method for updating user's FirstName, LastName, Email columns
func (r *Repository) Update(id uuid.UUID, firstName, lastName, email string) (error) {

	return r.Exec(func(conn *sql.DB) (error) {

		_, err := conn.Exec(r.updateQuery(), firstName, lastName, email, id)

		if err != nil {
			return err
		}

		return nil
	})
}

// Method for updating user's Password columns
func (r *Repository) UpdatePassword(id uuid.UUID, password string) (error) {

	return r.Exec(func(conn *sql.DB) (error) {

		_, err := conn.Exec(r.updatePasswordQuery(), password, id)

		if err != nil {
			return err
		}

		return nil
	})
}

// Method for creating table
func (r *Repository) CreateTable() (error) {
	return r.Exec(func(conn *sql.DB) (error) {

		_, err := conn.Exec(r.createTableQuery())

		if err != nil {
			return err
		}

		return nil
	})
}