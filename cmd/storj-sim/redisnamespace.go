// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.
package main

import (
	"strconv"
)

var redisDBs = map[string]int{
	"--live-accounting.storage-backend": 0,
	"--server.revocation-dburl":         1,
}

// createPath generates a redis path for the db provided
func createPath(hostPort string, db int) string {
	return "redis://" + hostPort + "?db=" + strconv.Itoa(db)
}
