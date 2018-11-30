// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"

	"storj.io/storj/pkg/satellite"
)

const (
	companyType      = "company"
	companyInputType = "companyInput"

	fieldUserID     = "userID"
	fieldName       = "name"
	fieldAddress    = "address"
	fieldCountry    = "country"
	fieldCity       = "city"
	fieldState      = "state"
	fieldPostalCode = "postalCode"
)

// graphqlCompany creates *graphql.Object type representation of satellite.Company
func graphqlCompany() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: companyType,
		Fields: graphql.Fields{
			fieldUserID: &graphql.Field{
				Type: graphql.String,
			},
			fieldName: &graphql.Field{
				Type: graphql.String,
			},
			fieldAddress: &graphql.Field{
				Type: graphql.String,
			},
			fieldCountry: &graphql.Field{
				Type: graphql.String,
			},
			fieldCity: &graphql.Field{
				Type: graphql.String,
			},
			fieldState: &graphql.Field{
				Type: graphql.String,
			},
			fieldPostalCode: &graphql.Field{
				Type: graphql.String,
			},
		},
	})
}

// graphqlCompanyInput creates graphql.InputObject type needed to register/update satellite.Company
func graphqlCompanyInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: companyInputType,
		Fields: graphql.InputObjectConfigFieldMap{
			fieldName: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldAddress: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldCountry: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldCity: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldState: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
			fieldPostalCode: &graphql.InputObjectFieldConfig{
				Type: graphql.String,
			},
		},
	})
}

// fromMapCompanyInfo creates satellite.CompanyInfo from input args
func fromMapCompanyInfo(args map[string]interface{}) (company satellite.CompanyInfo) {
	company.Name, _ = args[fieldName].(string)
	company.Address, _ = args[fieldAddress].(string)
	company.Country, _ = args[fieldCountry].(string)
	company.City, _ = args[fieldCity].(string)
	company.State, _ = args[fieldState].(string)
	company.PostalCode, _ = args[fieldPostalCode].(string)

	return
}

// fillCompanyInfo fills satellite.CompanyInfo from satellite.Company and input args
func fillCompanyInfo(company *satellite.Company, args map[string]interface{}) satellite.CompanyInfo {
	info := satellite.CompanyInfo{
		Name:       company.Name,
		Address:    company.Address,
		Country:    company.Country,
		City:       company.City,
		State:      company.State,
		PostalCode: company.PostalCode,
	}

	for fName, fValue := range args {
		val, ok := fValue.(string)
		if !ok {
			continue
		}

		switch fName {
		case fieldName:
			info.Name = val
			company.Name = val
		case fieldAddress:
			info.Address = val
			company.Address = val
		case fieldCountry:
			info.Country = val
			company.Country = val
		case fieldCity:
			info.City = val
			company.City = val
		case fieldState:
			info.State = val
			company.State = val
		case fieldPostalCode:
			info.PostalCode = val
			company.PostalCode = val
		}
	}

	return info
}
