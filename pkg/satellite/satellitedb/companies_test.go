// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/satellite"
	"storj.io/storj/pkg/satellite/satellitedb/dbx"
)

func TestCompanyRepository(t *testing.T) {
	//testing constants
	const (
		// for user
		lastName = "lastName"
		email    = "email@ukr.net"
		pass     = "123456"
		userName = "name"

		// for company
		companyName = "Storj"
		address     = "somewhere"
		country     = "USA"
		city        = "Atlanta"
		state       = "Georgia"
		postalCode  = "02183"

		// updated company values
		newCompanyName = "Storage"
		newAddress     = "where"
		newCountry     = "Usa"
		newCity        = "Otlanta"
		newState       = "Jeorgia"
		newPostalCode  = "02184"
	)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// creating in-memory db and opening connection
	// to test with real db3 file use this connection string - "../db/accountdb.db3"
	db, err := New("sqlite3", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(db.Close)

	// creating tables
	err = db.CreateTables()
	if err != nil {
		t.Fatal(err)
	}

	// repositories
	users := db.Users()
	companies := db.Companies()

	var user *satellite.User

	t.Run("Can't insert company without user", func(t *testing.T) {
		company := &satellite.Company{
			Name:       companyName,
			Address:    address,
			Country:    country,
			City:       city,
			State:      state,
			PostalCode: postalCode,
		}

		createdCompany, err := companies.Insert(ctx, company)

		assert.Nil(t, createdCompany)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("Insert company successfully", func(t *testing.T) {
		user, err = users.Insert(ctx, &satellite.User{
			FirstName:    userName,
			LastName:     lastName,
			Email:        email,
			PasswordHash: []byte(pass),
		})

		assert.NoError(t, err)
		assert.NotNil(t, user)

		company := &satellite.Company{
			UserID: user.ID,

			Name:       companyName,
			Address:    address,
			Country:    country,
			City:       city,
			State:      state,
			PostalCode: postalCode,
		}

		createdCompany, err := companies.Insert(ctx, company)

		assert.NotNil(t, createdCompany)
		assert.Nil(t, err)
		assert.NoError(t, err)
	})

	t.Run("Get company success", func(t *testing.T) {
		companyByUserID, err := companies.GetByUserID(ctx, user.ID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, companyByUserID.UserID, user.ID)
		assert.Equal(t, companyByUserID.Name, companyName)
		assert.Equal(t, companyByUserID.Address, address)
		assert.Equal(t, companyByUserID.Country, country)
		assert.Equal(t, companyByUserID.City, city)
		assert.Equal(t, companyByUserID.State, state)
		assert.Equal(t, companyByUserID.PostalCode, postalCode)

		companyByID, err := companies.GetByUserID(ctx, companyByUserID.UserID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		assert.Equal(t, companyByID.UserID, user.ID)
		assert.Equal(t, companyByID.Name, companyName)
		assert.Equal(t, companyByID.Address, address)
		assert.Equal(t, companyByID.Country, country)
		assert.Equal(t, companyByID.City, city)
		assert.Equal(t, companyByID.State, state)
		assert.Equal(t, companyByID.PostalCode, postalCode)
	})

	t.Run("Update company success", func(t *testing.T) {
		oldCompany, err := companies.GetByUserID(ctx, user.ID)

		assert.NoError(t, err)
		assert.NotNil(t, oldCompany)

		// creating new company with updated values
		newCompany := &satellite.Company{
			UserID:     user.ID,
			Name:       newCompanyName,
			Address:    newAddress,
			Country:    newCountry,
			City:       newCity,
			State:      newState,
			PostalCode: newPostalCode,
		}

		err = companies.Update(ctx, newCompany)

		assert.Nil(t, err)
		assert.NoError(t, err)

		// fetching updated company from db
		newCompany, err = companies.GetByUserID(ctx, oldCompany.UserID)

		assert.NoError(t, err)

		assert.Equal(t, newCompany.UserID, user.ID)
		assert.Equal(t, newCompany.Name, newCompanyName)
		assert.Equal(t, newCompany.Address, newAddress)
		assert.Equal(t, newCompany.Country, newCountry)
		assert.Equal(t, newCompany.City, newCity)
		assert.Equal(t, newCompany.State, newState)
		assert.Equal(t, newCompany.PostalCode, newPostalCode)
	})

	t.Run("Delete company success", func(t *testing.T) {
		oldCompany, err := companies.GetByUserID(ctx, user.ID)

		assert.NoError(t, err)
		assert.NotNil(t, oldCompany)

		err = companies.Delete(ctx, oldCompany.UserID)

		assert.Nil(t, err)
		assert.NoError(t, err)

		_, err = companies.GetByUserID(ctx, oldCompany.UserID)

		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}

func TestCompanyFromDbx(t *testing.T) {
	t.Run("can't create dbo from nil dbx model", func(t *testing.T) {
		company, err := companyFromDBX(nil)

		assert.Nil(t, company)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})

	t.Run("can't create dbo from dbx model with invalid UserID", func(t *testing.T) {
		dbxCompany := dbx.Company{
			UserId: []byte("qweqwe"),
		}

		company, err := companyFromDBX(&dbxCompany)

		assert.Nil(t, company)
		assert.NotNil(t, err)
		assert.Error(t, err)
	})
}
