// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package redisnamespace

import "strconv"

var dbs = map[string]int{
	"live-accounting.storage-backend": 0,
	"server.revocation-dburl":         1,
}

// getAll returns entire db map
func getAll() map[string]int {
	return dbs
}

// CreatePath generates a redis path for the db provided
func CreatePath(hostPort string, db int) string {
	return "redis://" + hostPort + "?db=" + strconv.Itoa(db)
}
