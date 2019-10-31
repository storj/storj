// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redisnamespace

import "strconv"

var dbs = map[string]int{
	"live-accounting.StorageBackend": 0,
	"server.revocation-dburl":        1,
}

// GetAll returns entire db map
func GetAll() map[string]int {
	return dbs
}

// GetDB returns the db value given the key
func GetDB(key string) int {
	return dbs[key]
}

// CreatePath generates a redis path for the db provided
func CreatePath(hostPort string, db int) string {
	return "redis://" + hostPort + "?db=" + strconv.Itoa(db)
}
