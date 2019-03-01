// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3" // register sqlite to sql
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
	// Error is the default psdb errs class
	Error = errs.Class("psdb")
)

// DB is a piece store database
type DB struct {
	mu sync.Mutex
	DB *sql.DB // TODO: hide
}

// Agreement is a struct that contains a bandwidth agreement and the associated signature
type Agreement struct {
	Agreement pb.Order
	Signature []byte
}

// Open opens DB at DBPath
func Open(DBPath string) (db *DB, err error) {
	if err = os.MkdirAll(filepath.Dir(DBPath), 0700); err != nil {
		return nil, err
	}

	sqlite, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?%s", DBPath, "_journal=WAL"))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	db = &DB{
		DB: sqlite,
	}

	return db, nil
}

// OpenInMemory opens sqlite DB inmemory
func OpenInMemory() (db *DB, err error) {
	sqlite, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	db = &DB{
		DB: sqlite,
	}

	return db, nil
}

// Migration define piecestore DB migration
func (db *DB) Migration() *migrate.Migration {
	migration := &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				Description: "Initial setup",
				Version:     0,
				Action: migrate.SQL{
					`CREATE TABLE IF NOT EXISTS ttl (
						id BLOB UNIQUE,
						created INT(10),
						expires INT(10),
						size INT(10)
					)`,
					`CREATE TABLE IF NOT EXISTS bandwidth_agreements (
						satellite BLOB,
						agreement BLOB,
						signature BLOB
					)`,
					`CREATE INDEX IF NOT EXISTS idx_ttl_expires ON ttl (
						expires
					)`,
					`CREATE TABLE IF NOT EXISTS bwusagetbl (
						size INT(10),
						daystartdate INT(10),
						dayenddate INT(10)
					)`,
				},
			},
			{
				Description: "Extending bandwidth_agreements table and drop bwusagetbl",
				Version:     1,
				Action: migrate.Func(func(log *zap.Logger, db migrate.DB, tx *sql.Tx) error {
					v1sql := migrate.SQL{
						`ALTER TABLE bandwidth_agreements ADD COLUMN uplink BLOB`,
						`ALTER TABLE bandwidth_agreements ADD COLUMN serial_num BLOB`,
						`ALTER TABLE bandwidth_agreements ADD COLUMN total INT(10)`,
						`ALTER TABLE bandwidth_agreements ADD COLUMN max_size INT(10)`,
						`ALTER TABLE bandwidth_agreements ADD COLUMN created_utc_sec INT(10)`,
						`ALTER TABLE bandwidth_agreements ADD COLUMN expiration_utc_sec INT(10)`,
						`ALTER TABLE bandwidth_agreements ADD COLUMN action INT(10)`,
						`ALTER TABLE bandwidth_agreements ADD COLUMN daystart_utc_sec INT(10)`,
					}
					err := v1sql.Run(log, db, tx)
					if err != nil {
						return err
					}

					// iterate through the table and fill
					err = func() error {
						rows, err := tx.Query(`SELECT agreement, signature FROM bandwidth_agreements`)
						if err != nil {
							return err
						}
						defer func() { err = errs.Combine(err, rows.Close()) }()

						for rows.Next() {
							var rbaBytes, signature []byte
							rba := &pb.RenterBandwidthAllocation{}
							err := rows.Scan(&rbaBytes, &signature)
							if err != nil {
								return err
							}
							// unmarshal the rbaBytes
							err = proto.Unmarshal(rbaBytes, rba)
							if err != nil {
								return err
							}
							// update the new columns data
							t := time.Unix(rba.PayerAllocation.CreatedUnixSec, 0)
							startofthedayUnixSec := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Unix()

							// update the row by signature as it is unique
							_, err = tx.Exec(`UPDATE bandwidth_agreements SET 
									uplink = ?,
									serial_num = ?,
									total = ?,
									max_size = ?,
									created_utc_sec = ?,
									expiration_utc_sec = ?,
									action = ?,
									daystart_utc_sec = ?
									WHERE signature = ?
								`,
								rba.PayerAllocation.UplinkId.Bytes(), rba.PayerAllocation.SerialNumber,
								rba.Total, rba.PayerAllocation.MaxSize, rba.PayerAllocation.CreatedUnixSec,
								rba.PayerAllocation.ExpirationUnixSec, rba.PayerAllocation.GetAction(),
								startofthedayUnixSec, signature)
							if err != nil {
								return err
							}
						}
						return rows.Err()
					}()
					if err != nil {
						return err
					}
					_, err = tx.Exec(`DROP TABLE bwusagetbl;`)
					if err != nil {
						return err
					}
					return nil
				}),
			},
		},
	}
	return migration
}

// Close the database
func (db *DB) Close() error {
	return db.DB.Close()
}

func (db *DB) locked() func() {
	db.mu.Lock()
	return db.mu.Unlock
}

// DeleteExpired deletes expired pieces
func (db *DB) DeleteExpired(ctx context.Context) (expired []string, err error) {
	defer mon.Task()(&ctx)(&err)
	defer db.locked()()

	// TODO: add limit

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()

	rows, err := tx.Query("SELECT id FROM ttl WHERE expires > 0 AND expires < ?", now)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		expired = append(expired, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	_, err = tx.Exec(`DELETE FROM ttl WHERE expires > 0 AND expires < ?`, now)
	if err != nil {
		return nil, err
	}

	return expired, tx.Commit()
}

// WriteBandwidthAllocToDB inserts bandwidth agreement into DB
func (db *DB) WriteBandwidthAllocToDB(rba *pb.Order) error {
	rbaBytes, err := proto.Marshal(rba)
	if err != nil {
		return err
	}
	defer db.locked()()

	// We begin extracting the satellite_id
	// The satellite id can be used to sort the bandwidth agreements
	// If the agreements are sorted we can send them in bulk streams to the satellite
	t := time.Now()
	startofthedayunixsec := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).Unix()
	_, err = db.DB.Exec(`INSERT INTO bandwidth_agreements (satellite, agreement, signature, uplink, serial_num, total, max_size, created_utc_sec, expiration_utc_sec, action, daystart_utc_sec) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rba.PayerAllocation.SatelliteId.Bytes(), rbaBytes, rba.GetSignature(),
		rba.PayerAllocation.UplinkId.Bytes(), rba.PayerAllocation.SerialNumber,
		rba.Total, rba.PayerAllocation.MaxSize, rba.PayerAllocation.CreatedUnixSec,
		rba.PayerAllocation.ExpirationUnixSec, rba.PayerAllocation.GetAction().String(),
		startofthedayunixsec)
	return err
}

// DeleteBandwidthAllocationBySerialnum finds an allocation by signature and deletes it
func (db *DB) DeleteBandwidthAllocationBySerialnum(serialnum string) error {
	defer db.locked()()
	_, err := db.DB.Exec(`DELETE FROM bandwidth_agreements WHERE serial_num=?`, serialnum)
	if err == sql.ErrNoRows {
		err = nil
	}
	return err
}

// GetBandwidthAllocationBySignature finds allocation info by signature
func (db *DB) GetBandwidthAllocationBySignature(signature []byte) ([]*pb.Order, error) {
	defer db.locked()()

	rows, err := db.DB.Query(`SELECT agreement FROM bandwidth_agreements WHERE signature = ?`, signature)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			zap.S().Errorf("failed to close rows when selecting from bandwidth_agreements: %+v", closeErr)
		}
	}()

	agreements := []*pb.Order{}
	for rows.Next() {
		var rbaBytes []byte
		err := rows.Scan(&rbaBytes)
		if err != nil {
			return agreements, err
		}
		rba := &pb.Order{}
		err = proto.Unmarshal(rbaBytes, rba)
		if err != nil {
			return agreements, err
		}
		agreements = append(agreements, rba)
	}
	return agreements, nil
}

// GetBandwidthAllocations all bandwidth agreements and sorts by satellite
func (db *DB) GetBandwidthAllocations() (map[storj.NodeID][]*Agreement, error) {
	defer db.locked()()

	rows, err := db.DB.Query(`SELECT satellite, agreement FROM bandwidth_agreements`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			zap.S().Errorf("failed to close rows when selecting from bandwidth_agreements: %+v", closeErr)
		}
	}()

	agreements := make(map[storj.NodeID][]*Agreement)
	for rows.Next() {
		rbaBytes := []byte{}
		agreement := &Agreement{}
		var satellite []byte
		err := rows.Scan(&satellite, &rbaBytes)
		if err != nil {
			return agreements, err
		}
		err = proto.Unmarshal(rbaBytes, &agreement.Agreement)
		if err != nil {
			return agreements, err
		}
		satelliteID, err := storj.NodeIDFromBytes(satellite)
		if err != nil {
			return nil, err
		}
		agreements[satelliteID] = append(agreements[satelliteID], agreement)
	}
	return agreements, nil
}

// AddTTL adds TTL into database by id
func (db *DB) AddTTL(id string, expiration, size int64) error {
	defer db.locked()()

	created := time.Now().Unix()
	_, err := db.DB.Exec("INSERT OR REPLACE INTO ttl (id, created, expires, size) VALUES (?, ?, ?, ?)", id, created, expiration, size)
	return err
}

// GetTTLByID finds the TTL in the database by id and return it
func (db *DB) GetTTLByID(id string) (expiration int64, err error) {
	defer db.locked()()

	err = db.DB.QueryRow(`SELECT expires FROM ttl WHERE id=?`, id).Scan(&expiration)
	return expiration, err
}

// SumTTLSizes sums the size column on the ttl table
func (db *DB) SumTTLSizes() (sum int64, err error) {
	defer db.locked()()

	var count int
	rows := db.DB.QueryRow("SELECT COUNT(*) as count FROM ttl")
	err = rows.Scan(&count)
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	err = db.DB.QueryRow(`SELECT SUM(size) FROM ttl;`).Scan(&sum)
	return sum, err
}

// DeleteTTLByID finds the TTL in the database by id and delete it
func (db *DB) DeleteTTLByID(id string) error {
	defer db.locked()()

	_, err := db.DB.Exec(`DELETE FROM ttl WHERE id=?`, id)
	if err == sql.ErrNoRows {
		err = nil
	}
	return err
}

// GetBandwidthUsedByDay finds the so far bw used by day and return it
func (db *DB) GetBandwidthUsedByDay(t time.Time) (size int64, err error) {
	return db.GetTotalBandwidthBetween(t, t)
}

// GetTotalBandwidthBetween each row in the bwusagetbl contains the total bw used per day
func (db *DB) GetTotalBandwidthBetween(startdate time.Time, enddate time.Time) (totalbwusage int64, err error) {
	defer db.locked()()

	startTimeUnix := time.Date(startdate.Year(), startdate.Month(), startdate.Day(), 0, 0, 0, 0, startdate.Location()).Unix()
	endTimeUnix := time.Date(enddate.Year(), enddate.Month(), enddate.Day(), 24, 0, 0, 0, enddate.Location()).Unix()
	defaultunixtime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location()).Unix()

	if (endTimeUnix < startTimeUnix) && (startTimeUnix > defaultunixtime || endTimeUnix > defaultunixtime) {
		return totalbwusage, errors.New("Invalid date range")
	}

	var count int
	rows := db.DB.QueryRow(`SELECT COUNT(*) as count FROM bandwidth_agreements WHERE daystart_utc_sec BETWEEN ? AND ?`, startTimeUnix, endTimeUnix)
	err = rows.Scan(&count)
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	err = db.DB.QueryRow(`SELECT SUM(total) FROM bandwidth_agreements WHERE daystart_utc_sec BETWEEN ? AND ?`, startTimeUnix, endTimeUnix).Scan(&totalbwusage)
	return totalbwusage, err
}

// Begin begins transaction
func (db *DB) Begin() (*sql.Tx, error) { return db.DB.Begin() }

// Rebind rebind parameters
func (db *DB) Rebind(s string) string { return s }

// Schema returns schema
func (db *DB) Schema() string { return "" }
