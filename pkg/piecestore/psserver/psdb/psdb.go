// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3" // register sqlite to sql
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/migrate"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var (
	// Error is the default psdb errs class
	Error = errs.Class("psdb")
)

// AgreementStatus keep tracks of the agreement payout status
type AgreementStatus int32

const (
	// AgreementStatusUnsent sets the agreement status to UNSENT
	AgreementStatusUnsent = iota
	// AgreementStatusSent  sets the agreement status to SENT
	AgreementStatusSent
	// AgreementStatusReject sets the agreement status to REJECT
	AgreementStatusReject
	// add new status here ...
)

// DB is a piece store database
type DB struct {
	mu     sync.Mutex
	db     *sql.DB
	dbPath string
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
		db:     sqlite,
		dbPath: DBPath,
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
		db: sqlite,
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
			{
				Description: "Add status column for bandwidth_agreements",
				Version:     2,
				Action: migrate.SQL{
					`ALTER TABLE bandwidth_agreements ADD COLUMN status INT(10) DEFAULT 0`,
				},
			},
			{
				Description: "Add index on serial number for bandwidth_agreements",
				Version:     3,
				Action: migrate.SQL{
					`CREATE INDEX IF NOT EXISTS idx_bwa_serial ON bandwidth_agreements (serial_num)`,
				},
			},
			{
				Description: "Initiate Network reset",
				Version:     4,
				Action: migrate.SQL{
					`UPDATE ttl SET expires = 1553727600 WHERE created <= 1553727600 `,
				},
			},
			{
				Description: "delete obsolete pieces",
				Version:     5,
				Action: migrate.Func(func(log *zap.Logger, mdb migrate.DB, tx *sql.Tx) error {
					path := db.dbPath
					if path == "" {
						log.Warn("Empty path")
						return nil
					}
					return db.DeleteObsolete(path)
				}),
			},
		},
	}
	return migration
}

// Close the database
func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) locked() func() {
	db.mu.Lock()
	return db.mu.Unlock
}

// DeleteObsolete deletes obsolete pieces
func (db *DB) DeleteObsolete(path string) (err error) {
	path = filepath.Dir(path)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	// iterate thru files list
	for _, f := range files {
		if info, err := os.Stat(filepath.Join(path, f.Name())); err == nil && info.IsDir() && len(f.Name()) == 2 {
			err = os.RemoveAll(filepath.Join(path, f.Name()))
			if err != nil {
				return err
			}
		}
	}
	return nil
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
	_, err = db.db.Exec(`INSERT INTO bandwidth_agreements (satellite, agreement, signature, uplink, serial_num, total, max_size, created_utc_sec, status, expiration_utc_sec, action, daystart_utc_sec) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rba.PayerAllocation.SatelliteId.Bytes(), rbaBytes, rba.GetSignature(),
		rba.PayerAllocation.UplinkId.Bytes(), rba.PayerAllocation.SerialNumber,
		rba.Total, rba.PayerAllocation.MaxSize, rba.PayerAllocation.CreatedUnixSec, AgreementStatusUnsent,
		rba.PayerAllocation.ExpirationUnixSec, rba.PayerAllocation.GetAction().String(),
		startofthedayunixsec)
	return err
}

// DeleteBandwidthAllocationPayouts delete paid and/or old payout enteries based on days old
func (db *DB) DeleteBandwidthAllocationPayouts() error {
	defer db.locked()()

	//@TODO make a config value for older days
	t := time.Now().Add(time.Hour * 24 * -90).Unix()
	_, err := db.db.Exec(`DELETE FROM bandwidth_agreements WHERE created_utc_sec < ?`, t)
	if err == sql.ErrNoRows {
		err = nil
	}
	return err
}

// UpdateBandwidthAllocationStatus update the bwa payout status
func (db *DB) UpdateBandwidthAllocationStatus(serialnum string, status AgreementStatus) (err error) {
	defer db.locked()()
	_, err = db.db.Exec(`UPDATE bandwidth_agreements SET status = ? WHERE serial_num = ?`, status, serialnum)
	return err
}

// DeleteBandwidthAllocationBySerialnum finds an allocation by signature and deletes it
func (db *DB) DeleteBandwidthAllocationBySerialnum(serialnum string) error {
	defer db.locked()()
	_, err := db.db.Exec(`DELETE FROM bandwidth_agreements WHERE serial_num=?`, serialnum)
	if err == sql.ErrNoRows {
		err = nil
	}
	return err
}

// GetBandwidthAllocationBySignature finds allocation info by signature
func (db *DB) GetBandwidthAllocationBySignature(signature []byte) ([]*pb.Order, error) {
	defer db.locked()()

	rows, err := db.db.Query(`SELECT agreement FROM bandwidth_agreements WHERE signature = ?`, signature)
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

// GetBandwidthAllocations all bandwidth agreements
func (db *DB) GetBandwidthAllocations() (map[storj.NodeID][]*Agreement, error) {
	defer db.locked()()

	rows, err := db.db.Query(`SELECT satellite, agreement FROM bandwidth_agreements WHERE status = ?`, AgreementStatusUnsent)
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

// GetBwaStatusBySerialNum get BWA status by serial num
func (db *DB) GetBwaStatusBySerialNum(serialnum string) (status AgreementStatus, err error) {
	defer db.locked()()
	err = db.db.QueryRow(`SELECT status FROM bandwidth_agreements WHERE serial_num=?`, serialnum).Scan(&status)
	return status, err
}

// AddTTL adds TTL into database by id
func (db *DB) AddTTL(id string, expiration, size int64) error {
	defer db.locked()()

	created := time.Now().Unix()
	_, err := db.db.Exec("INSERT OR REPLACE INTO ttl (id, created, expires, size) VALUES (?, ?, ?, ?)", id, created, expiration, size)
	return err
}

// GetTTLByID finds the TTL in the database by id and return it
func (db *DB) GetTTLByID(id string) (expiration int64, err error) {
	defer db.locked()()

	err = db.db.QueryRow(`SELECT expires FROM ttl WHERE id=?`, id).Scan(&expiration)
	return expiration, err
}

// SumTTLSizes sums the size column on the ttl table
func (db *DB) SumTTLSizes() (int64, error) {
	defer db.locked()()

	var sum *int64
	err := db.db.QueryRow(`SELECT SUM(size) FROM ttl;`).Scan(&sum)
	if err == sql.ErrNoRows || sum == nil {
		return 0, nil
	}
	return *sum, err
}

// DeleteTTLByID finds the TTL in the database by id and delete it
func (db *DB) DeleteTTLByID(id string) error {
	defer db.locked()()

	_, err := db.db.Exec(`DELETE FROM ttl WHERE id=?`, id)
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
func (db *DB) GetTotalBandwidthBetween(startdate time.Time, enddate time.Time) (int64, error) {
	defer db.locked()()

	startTimeUnix := time.Date(startdate.Year(), startdate.Month(), startdate.Day(), 0, 0, 0, 0, startdate.Location()).Unix()
	endTimeUnix := time.Date(enddate.Year(), enddate.Month(), enddate.Day(), 24, 0, 0, 0, enddate.Location()).Unix()
	defaultunixtime := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location()).Unix()

	if (endTimeUnix < startTimeUnix) && (startTimeUnix > defaultunixtime || endTimeUnix > defaultunixtime) {
		return 0, errors.New("Invalid date range")
	}

	var totalUsage *int64
	err := db.db.QueryRow(`SELECT SUM(total) FROM bandwidth_agreements WHERE daystart_utc_sec BETWEEN ? AND ?`, startTimeUnix, endTimeUnix).Scan(&totalUsage)
	if err == sql.ErrNoRows || totalUsage == nil {
		return 0, nil
	}
	return *totalUsage, err
}

// RawDB returns access to the raw database, only for migration tests.
func (db *DB) RawDB() *sql.DB { return db.db }

// Begin begins transaction
func (db *DB) Begin() (*sql.Tx, error) { return db.db.Begin() }

// Rebind rebind parameters
func (db *DB) Rebind(s string) string { return s }

// Schema returns schema
func (db *DB) Schema() string { return "" }
