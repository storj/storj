// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redisnamespace

import "strconv"

var dbs = map[string]int{
	"live-accounting.db":      0,
	"server.revocation-dburl": 1,
}

// GetDB returns the database value of the key provided.
func GetDB(key string) int {
	return dbs[key]
}

// GetKeys returns all the keys in the map
func GetKeys() (keys []string) {
	for key := range dbs {
		keys = append(keys, key)
	}
	return keys
}

// GetAll returns entire db map
func GetAll() map[string]int {
	return dbs
}

// CreatePath generates a redis path for the db provided
func CreatePath(main string, db int) string {
	return main + "?db=" + strconv.Itoa(db)
}
