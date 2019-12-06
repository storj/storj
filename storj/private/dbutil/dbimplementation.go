// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

// Implementation type of valid DBs
type Implementation int

const (
	// Unknown is an unknown db type
	Unknown Implementation = iota
	// Postgres is a Postgresdb type
	Postgres
	// Cockroach is a Cockroachdb type
	Cockroach
	// Bolt is a Bolt kv store
	Bolt
	// Redis is a Redis kv store
	Redis
)

func setImplementation(s string) Implementation {
	switch s {
	case "postgres":
		return Postgres
	case "cockroach":
		return Cockroach
	case "bolt":
		return Bolt
	case "redis":
		return Redis
	case "postgresql":
		return Postgres
	default:
		return Unknown
	}
}
