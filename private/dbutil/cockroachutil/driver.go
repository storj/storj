// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil

import (
	"database/sql"
	"database/sql/driver"
	"strings"

	"github.com/lib/pq"
)

// Driver is the type for the "cockroach" sql/database driver.
// It uses github.com/lib/pq under the covers because of Cockroach's
// PostgreSQL compatibility, but allows differentiation between pg and
// crdb connections.
type Driver struct {
	pq.Driver
}

// Open opens a new cockroachDB connection.
func (cd *Driver) Open(name string) (driver.Conn, error) {
	return Open(name)
}

// Open opens a new cockroachDB connection.
func Open(name string) (driver.Conn, error) {
	name = translateName(name)
	return pq.Open(name)
}

// OpenConnector obtains a new db Connector, which sql.DB can use to
// obtain each needed connection at the appropriate time.
func (cd *Driver) OpenConnector(name string) (driver.Connector, error) {
	name = translateName(name)
	pgConnector, err := pq.NewConnector(name)
	if err != nil {
		return nil, err
	}
	return &Connector{pgConnector}, nil
}

// translateName changes the scheme name in a `cockroach://` URL to
// `postgres://`, as that is what lib/pq will expect.
func translateName(name string) string {
	if strings.HasPrefix(name, "cockroach://") {
		name = "postgres://" + name[12:]
	}
	return name
}

// Connector is a thin wrapper around a pq-based connector. This allows
// Driver to satisfy driver.DriverContext, and avoids weird breakage if
// and when we upgrade from pq 1.0 to pq 1.2 or higher.
type Connector struct {
	driver.Connector
}

// Driver returns the driver being used for this connector.
func (conn *Connector) Driver() driver.Driver {
	return &Driver{}
}

var _ driver.DriverContext = &Driver{}

func init() {
	sql.Register("cockroach", &Driver{})
}
