// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"fmt"
	"strconv"
	"time"
)

// Implementation type of valid DBs.
//
//enumcheck:complete
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
	// TiDB is a TiDB instance speaking the MySQL wire protocol.
	TiDB
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
	case "tidb":
		return TiDB
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
	case TiDB:
		return "tidb"
	case Unknown:
		fallthrough
	default:
		return "<unknown>"
	}
}

// AsOfSystemTime returns a SQL query for the specifying the AS OF SYSTEM TIME using
// a concrete time.
func (impl Implementation) AsOfSystemTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	switch impl {
	case Cockroach:
		return " AS OF SYSTEM TIME '" + strconv.FormatInt(t.UnixNano(), 10) + "' "
	case TiDB:
		return " AS OF TIMESTAMP '" + t.UTC().Format("2006-01-02 15:04:05.000000-07:00") + "' "
	default:
		return ""
	}
}

// AsOfSystemTimeBounded returns a SQL query for the specifying the AS OF SYSTEM TIME using
// a concrete bounded time. This allows to query the most recent data available within the specified time.
func (impl Implementation) AsOfSystemTimeBounded(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	switch impl {
	case Cockroach:
		return impl.AsOfSystemTime(t)
	case TiDB:
		return " AS OF TIMESTAMP TIDB_BOUNDED_STALENESS('" + t.UTC().Format("2006-01-02 15:04:05.000000-07:00") + "', NOW(6)) "
	default:
		return ""
	}
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

// tidbMinAsOfInterval is the smallest staleness for which a TiDB stale read is
// reliable.
//
// TiDB orders all transactions using timestamps from a single authority: the
// Timestamp Oracle (TSO) run by PD (Placement Driver), the cluster's
// coordinator. A regular read fetches a fresh timestamp from the TSO, so it is
// always valid. A stale read (AS OF TIMESTAMP) instead lets us name the read
// timestamp ourselves via NOW(), which is evaluated on the tidb-server's local
// wall clock rather than the TSO. If that clock runs ahead of the TSO by the
// inter-node clock skew, a staleness smaller than the skew names a timestamp
// that is still in the TSO's future, which TiDB rejects with "cannot set read
// timestamp to a future time".
//
// Sub-second stale reads are therefore unreliable (and not meaningful) on TiDB,
// so below this threshold we fall back to a consistent (latest) read instead.
const tidbMinAsOfInterval = time.Second

// MinAsOfSystemInterval returns the smallest staleness for which an AS OF SYSTEM
// TIME read is reliable on this implementation. A stale read closer to now than
// this should fall back to a consistent read. Only TiDB has a non-zero minimum
// (see tidbMinAsOfInterval); other databases can read arbitrarily close to now.
func (impl Implementation) MinAsOfSystemInterval() time.Duration {
	if impl == TiDB {
		return tidbMinAsOfInterval
	}
	return 0
}

// AsOfSystemInterval returns a SQL query for the specifying the AS OF SYSTEM TIME using
// a relative interval. The interval should be negative.
func (impl Implementation) AsOfSystemInterval(interval time.Duration) string {
	// a positive or zero interval disables AS OF SYSTEM TIME.
	if interval >= 0 {
		return ""
	}

	// Intervals below -1µs are not supported.
	if interval > -time.Microsecond {
		interval = -time.Microsecond
	}

	switch impl {
	case Cockroach:
		return " AS OF SYSTEM TIME '" + interval.String() + "' "
	case TiDB:
		if -interval < tidbMinAsOfInterval {
			return ""
		}
		return fmt.Sprintf(" AS OF TIMESTAMP NOW(6) - INTERVAL %d MICROSECOND ", -interval.Microseconds())
	default:
		return ""
	}
}

// AsOfSystemIntervalBounded returns a SQL query for the specifying the AS OF SYSTEM TIME using
// a relative interval using a bounded staleness. This allows to query the most recent data
// available within the specified interval. The interval should be negative.
func (impl Implementation) AsOfSystemIntervalBounded(interval time.Duration) string {
	// a positive or zero interval disables AS OF SYSTEM TIME.
	if interval >= 0 {
		return ""
	}

	// Intervals below -1µs are not supported.
	if interval > -time.Microsecond {
		interval = -time.Microsecond
	}

	switch impl {
	case Cockroach:
		return impl.AsOfSystemInterval(interval)
	case TiDB:
		if -interval < tidbMinAsOfInterval {
			return ""
		}
		return fmt.Sprintf(" AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(NOW(6) - INTERVAL %d MICROSECOND, NOW(6)) ", -interval.Microseconds())
	default:
		return ""
	}
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

// Float64Type returns the type name for the given implementation.
func (impl Implementation) Float64Type() string {
	switch impl {
	case Postgres, Cockroach, TiDB:
		return "FLOAT"
	case Spanner:
		return "FLOAT64"
	case SQLite3:
		return "REAL"
	case Unknown:
		fallthrough
	default:
		panic("unsupported Float64Type")
	}
}
