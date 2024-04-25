// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"strconv"
	"time"
)

// Implementation type of valid DBs.
type Implementation int

const (
	// Unknown is an unknown db type.
	Unknown Implementation = iota
	// Postgres is a Postgresdb type.
	Postgres
	// Cockroach is a Cockroachdb type.
	Cockroach
	// Bolt is a Bolt kv store.
	Bolt
	// Redis is a Redis kv store.
	Redis
	// SQLite3 is a sqlite3 database.
	SQLite3
	// Spanner is Google Spanner instance with Google SQL dialect.
	Spanner
)

// ImplementationForScheme returns the Implementation that is used for
// the url with the provided scheme.
func ImplementationForScheme(scheme string) Implementation {
	switch scheme {
	case "pgx", "postgres", "postgresql":
		return Postgres
	case "cockroach":
		return Cockroach
	case "bolt":
		return Bolt
	case "redis":
		return Redis
	case "sqlite", "sqlite3":
		return SQLite3
	case "spanner":
		return Spanner
	default:
		return Unknown
	}
}

// SchemeForImplementation returns the scheme that is used for URLs
// that use the given Implementation.
func SchemeForImplementation(implementation Implementation) string {
	return implementation.String()
}

// String returns the default name for a given implementation.
func (impl Implementation) String() string {
	switch impl {
	case Postgres:
		return "postgres"
	case Cockroach:
		return "cockroach"
	case Bolt:
		return "bolt"
	case Redis:
		return "redis"
	case SQLite3:
		return "sqlite3"
	case Spanner:
		return "spanner"
	default:
		return "<unknown>"
	}
}

// AsOfSystemTime returns a SQL query for the specifying the AS OF SYSTEM TIME using
// a concrete time.
func (impl Implementation) AsOfSystemTime(t time.Time) string {
	if impl != Cockroach {
		return ""
	}
	if t.IsZero() {
		return ""
	}
	return " AS OF SYSTEM TIME '" + strconv.FormatInt(t.UnixNano(), 10) + "' "
}

// WrapAsOfSystemTime converts a query to include AS OF SYSTEM TIME using
// a concrete time.
func (impl Implementation) WrapAsOfSystemTime(sql string, t time.Time) string {
	aost := impl.AsOfSystemTime(t)
	if aost == "" {
		return sql
	}
	return "SELECT * FROM (" + sql + ")" + aost
}

// AsOfSystemInterval returns a SQL query for the specifying the AS OF SYSTEM TIME using
// a relative interval. The interval should be negative.
func (impl Implementation) AsOfSystemInterval(interval time.Duration) string {
	if impl != Cockroach {
		return ""
	}

	// a positive or zero interval disables AS OF SYSTEM TIME.
	if interval >= 0 {
		return ""
	}

	// Cockroach does not support intervals smaller than a microsecond.
	if interval > -time.Microsecond {
		interval = -time.Microsecond
	}

	return " AS OF SYSTEM TIME '" + interval.String() + "' "
}

// WrapAsOfSystemInterval converts a query to include AS OF SYSTEM TIME using
// a relative interval. The interval should be negative.
func (impl Implementation) WrapAsOfSystemInterval(sql string, interval time.Duration) string {
	aost := impl.AsOfSystemInterval(interval)
	if aost == "" {
		return sql
	}
	return "SELECT * FROM (" + sql + ")" + aost
}
