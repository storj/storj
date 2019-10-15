// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package partners implements partners management for attributions.
package partners

import (
	"encoding/json"
	"os"

	"github.com/zeebo/errs"
)

// List defines a json struct for defining partners.
type List struct {
	Partners []Partner
}

// ListFromJSONFile loads a json definition of partners.
func ListFromJSONFile(path string) (*List, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, Error.Wrap(file.Close()))
	}()

	var list List
	err = json.NewDecoder(file).Decode(&list)
	return &list, Error.Wrap(err)
}

// StaticDB implements partner lookup based on a static definition.
type StaticDB struct {
	list        *List
	byName      map[string][]Partner
	byID        map[string]Partner
	byUserAgent map[string]Partner
}

// NewStaticDB creates a new StaticDB.
func NewStaticDB(list *List) (*StaticDB, error) {
	db := &StaticDB{
		list:        list,
		byName:      map[string][]Partner{},
		byID:        map[string]Partner{},
		byUserAgent: map[string]Partner{},
	}

	var errg errs.Group
	for _, p := range list.Partners {
		db.byName[p.Name] = append(db.byName[p.Name], p)

		if _, exists := db.byID[p.ID]; exists {
			errg.Add(Error.New("id %q already exists", p.ID))
		} else {
			db.byID[p.ID] = p
		}

		useragent := CanonicalUserAgent(p.UserAgent())
		if _, exists := db.byUserAgent[useragent]; exists {
			errg.Add(Error.New("user agent %q already exists", useragent))
		} else {
			db.byUserAgent[useragent] = p
		}
	}

	return db, errg.Err()
}
