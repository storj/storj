// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers the "mysql" driver used to talk to the cluster's TiDB
	pd "github.com/tikv/pd/client"
	"github.com/tikv/pd/client/pkg/caller"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil"
)

// sqlDSNEnv points at the SQL port of the same cluster as STORJ_TEST_TIDB_PD,
// e.g. "root@tcp(127.0.0.1:4500)/".
const sqlDSNEnv = "STORJ_TEST_TIDB_SQL"

// requireSQL skips the test unless the cluster's TiDB SQL port is configured.
func requireSQL(t *testing.T) string {
	dsn := os.Getenv(sqlDSNEnv)
	if dsn == "" {
		t.Skipf("%s is not set; start a cluster with "+
			"`docker compose -f testsuite/docker-compose.tidb-cluster.yaml up -d` "+
			"and set %s='root@tcp(127.0.0.1:4500)/'", sqlDSNEnv, sqlDSNEnv)
	}
	return dsn
}

// TestSafepointIntegration_ConsistentScanUnderConcurrentDML is the end-to-end
// claim the Holder exists for: on a real cluster, a multi-statement scan
// AS OF holder.ReadTime() -- rendered exactly as production renders it, via
// dbutil.TiDB.AsOfSystemTime -- returns one stable snapshot while rows are
// inserted and deleted underneath it, with a real GC safe point standing at
// the barrier.
//
// Why this is not vacuous:
//   - The GC safe point is moved up to *exactly* the barrier before the scan
//     runs (see armGCSafePoint), so the scan's own read timestamp is the oldest
//     one the cluster still permits. Any mismatch between where Hold registers
//     the barrier and where AS OF ReadTime() actually reads -- e.g. registering
//     at physical<<18|logical while TiDB reads physical<<18, the bug this test
//     was written for -- makes every batch fail with ERROR 9006.
//   - Arming is confirmed, not assumed: a probe read one millisecond *below*
//     the barrier must be rejected with 9006 before the real scan starts. If
//     the safe point were not enforced, that probe would succeed and the test
//     fails right there rather than passing for the wrong reason.
//   - The mutations are confirmed to have landed: a live read after the scan
//     must show the inserts present and the deletes gone. A mutator that did
//     nothing would make a stable snapshot trivial, so it fails the test.
//   - The scan is batched with a keyset cursor, so consistency has to hold
//     across many separate statements, not within one.
func TestSafepointIntegration_ConsistentScanUnderConcurrentDML(t *testing.T) {
	endpoint := requirePD(t)
	dsn := requireSQL(t)
	base := pdHTTPBase(endpoint)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	suffix, err := uuid.New()
	if err != nil {
		t.Fatal(err)
	}
	dbName := "storj_gc_snapshot_" + suffix.String()[:8]
	table := dbName + ".entries"

	if _, err := db.ExecContext(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), "DROP DATABASE "+dbName); err != nil {
			t.Errorf("dropping %s: %v", dbName, err)
		}
	}()

	if _, err := db.ExecContext(ctx, "CREATE TABLE "+table+" (id BIGINT PRIMARY KEY, payload VARCHAR(32) NOT NULL)"); err != nil {
		t.Fatal(err)
	}

	// The original rows: everything the snapshot must contain, no more, no less.
	const originalRows = 200
	for id := 1; id <= originalRows; id++ {
		if _, err := db.ExecContext(ctx, "INSERT INTO "+table+" (id, payload) VALUES (?, ?)", id, "original"); err != nil {
			t.Fatal(err)
		}
	}

	// A stale read taken immediately after the DDL can still miss the table
	// ("table doesn't exist") while the schema version settles; the barrier must
	// land after that, or the scan fails for a reason that has nothing to do
	// with GC. Observed on v8.5.6.
	time.Sleep(time.Second)

	holder, err := Hold(ctx, log, SafepointConfig{
		PDEndpoints: endpoint,
		// deliberately not the fast test's prefix, whose leftover check would
		// otherwise trip over this hold
		ServiceID: "storj-gc-snapshot",
		TTL:       10 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Release(ctx) }()

	readTime := holder.ReadTime()
	asOf := dbutil.TiDB.AsOfSystemTime(readTime)
	t.Logf("holding %q at %v; AS OF clause: %s", holder.ServiceID(), readTime, asOf)

	barrier := armGCSafePoint(ctx, t, endpoint, base, holder)

	// TiDB polls the safe point, so wait until it is enforcing it -- and prove
	// it is, by requiring a read below the barrier to be rejected.
	enforcedCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := waitGCEnforced(enforcedCtx, db, table, readTime); err != nil {
		t.Fatal(err)
	}

	// Concurrent DML: inserts (must be invisible to the snapshot) and deletes of
	// original rows (must still be visible in it).
	const mutations = 50
	firstMutation := make(chan struct{})
	mutatorDone := make(chan struct{})
	ctx.Go(func() error {
		defer close(mutatorDone)
		for i := 0; i < mutations; i++ {
			if _, err := db.ExecContext(ctx, "INSERT INTO "+table+" (id, payload) VALUES (?, ?)", originalRows+1+i, "inserted"); err != nil {
				return errs.Wrap(err)
			}
			if _, err := db.ExecContext(ctx, "DELETE FROM "+table+" WHERE id = ?", 1+i); err != nil {
				return errs.Wrap(err)
			}
			if i == 0 {
				close(firstMutation)
			}
			time.Sleep(10 * time.Millisecond)
		}
		return nil
	})

	// Make sure the snapshot is genuinely reading over mutated data rather than
	// racing ahead of the first one.
	select {
	case <-firstMutation:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// The scan itself: batched with a keyset cursor so that every batch is its
	// own statement resolving AS OF the same timestamp, interleaved with the DML
	// above.
	var scanned []int64
	cursor := int64(0)
	for batch := 0; ; batch++ {
		rows, err := db.QueryContext(ctx, "SELECT id, payload FROM "+table+asOf+
			"WHERE id > ? ORDER BY id LIMIT 20", cursor)
		if err != nil {
			t.Fatalf("batch %d (cursor %d) failed at safe point %d: %v", batch, cursor, barrier, err)
		}
		count := 0
		for rows.Next() {
			var id int64
			var payload string
			if err := rows.Scan(&id, &payload); err != nil {
				t.Fatal(errs.Combine(err, rows.Close()))
			}
			if payload != "original" {
				t.Errorf("row %d has payload %q: the snapshot saw a concurrent write", id, payload)
			}
			scanned = append(scanned, id)
			cursor = id
			count++
		}
		if err := errs.Combine(rows.Err(), rows.Close()); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			break
		}
		// give the mutator room to run between batches
		time.Sleep(20 * time.Millisecond)
	}

	select {
	case <-mutatorDone:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// The snapshot is exactly the original rows: nothing inserted since is
	// visible, and nothing deleted since has disappeared.
	if len(scanned) != originalRows {
		t.Fatalf("scanned %d rows, expected the %d original ones", len(scanned), originalRows)
	}
	for i, id := range scanned {
		if id != int64(i+1) {
			t.Fatalf("scanned row %d is id %d, expected %d", i, id, i+1)
		}
	}

	// The mutator actually changed the table -- otherwise a stable snapshot
	// would prove nothing at all.
	var live, liveInserted int
	if err := db.QueryRowContext(ctx, "SELECT count(*), sum(payload = 'inserted') FROM "+table).Scan(&live, &liveInserted); err != nil {
		t.Fatal(err)
	}
	if liveInserted != mutations || live != originalRows {
		t.Fatalf("live table has %d rows of which %d inserted; expected %d rows with %d inserted "+
			"(%d originals deleted): the concurrent DML did not happen",
			live, liveInserted, originalRows, mutations, mutations)
	}
	t.Logf("scan returned the %d original rows unchanged while %d rows were inserted and %d deleted",
		len(scanned), mutations, mutations)
}

// armGCSafePoint makes the holder's barrier the cluster's GC safe point, so
// that the snapshot the holder protects is the oldest one TiDB still allows a
// read at, and returns it.
//
// Left alone this only happens once a scan outlives gc_life_time (10 minutes,
// the floor TiDB accepts): gc_worker asks PD to collect up to now-10m, PD hands
// back the minimum across services -- our barrier -- and gc_worker publishes
// that. Waiting for it is what TestSafepointIntegration_GCWithheldAcrossCycles
// does, at 25 minutes a run. Here we drive the same sequence by hand instead:
// push gc_worker's own service safe point past the barrier so that PD's minimum
// is ours, then publish that minimum exactly the way gc_worker publishes it. PD
// deciding the minimum is real; only gc_worker's tick is simulated.
//
// This permanently pushes gc_worker forward -- PD allows neither lowering nor
// deleting its entry -- so the cluster is disposable afterwards.
func armGCSafePoint(ctx *testcontext.Context, t *testing.T, endpoint, base string, holder *Holder) uint64 {
	client, err := pd.NewClientWithContext(ctx, caller.Component("storj/gc-safepoint-test"),
		strings.Split(endpoint, ","), pd.SecurityOption{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	status, err := readGCSafepoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	ours, ok := status.find(holder.ServiceID())
	if !ok {
		t.Fatalf("no service safepoint registered for %q; got %+v",
			holder.ServiceID(), status.ServiceGCSafePoints)
	}

	// PD requires gc_worker's TTL to be infinity. One tick past our hold is the
	// smallest push that makes our barrier the binding minimum.
	//lint:ignore SA1019 the barrier API that replaces this does not exist on PD v8.5.x, which is what Hold falls back to.
	//nolint:staticcheck // the barrier API that replaces this does not exist on PD v8.5.x, which is what Hold falls back to.
	if _, err := client.UpdateServiceGCSafePoint(ctx, "gc_worker", math.MaxInt64, ours.SafePoint+1); err != nil {
		t.Fatal(err)
	}

	// What gc_worker would now be told when it asks to collect up to a
	// timestamp past our hold: the minimum, which is our barrier.
	//lint:ignore SA1019 deprecated is fine here; see above.
	//nolint:staticcheck // deprecated is fine here; see above.
	min, err := client.UpdateServiceGCSafePoint(ctx, "storj-gc-snapshot-competitor", 600,
		ours.SafePoint+uint64(time.Hour/time.Millisecond)<<tsoPhysicalShiftBits)
	if err != nil {
		t.Fatal(err)
	}
	if min != ours.SafePoint {
		t.Fatalf("PD reported minimum %d (%v), expected our barrier %d (%v)",
			min, tsoWall(min), ours.SafePoint, tsoWall(ours.SafePoint))
	}

	// Publish it the way gc_worker does at the end of a run.
	//lint:ignore SA1019 deprecated is fine here; see above.
	//nolint:staticcheck // deprecated is fine here; see above.
	if _, err := client.UpdateGCSafePoint(ctx, min); err != nil {
		t.Fatal(err)
	}
	if err := saveTiDBSafePoint(ctx, base, min); err != nil {
		t.Fatal(err)
	}

	armed, err := readGCSafepoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if armed.GCSafePoint != ours.SafePoint {
		t.Fatalf("cluster gc_safe_point is %d (%v), expected our barrier %d (%v)",
			armed.GCSafePoint, tsoWall(armed.GCSafePoint), ours.SafePoint, tsoWall(ours.SafePoint))
	}
	t.Logf("cluster gc_safe_point armed at our barrier %d (%v)", ours.SafePoint, tsoWall(ours.SafePoint))
	return ours.SafePoint
}

// saveTiDBSafePoint writes the GC safe point to the etcd key TiDB's KV store
// polls, which is what actually gates reads: PD's own gc_safe_point does not
// reach TiDB (verified on v8.5.6 -- setting it alone leaves stale reads below
// it succeeding). gc_worker writes this key at the end of every GC run; a
// TiDB picks it up within a few seconds.
//
// PD embeds etcd and serves its v3 HTTP gateway on the same client port, so no
// etcd client is needed for the one Put.
func saveTiDBSafePoint(ctx context.Context, base string, safePoint uint64) (err error) {
	// the key and value encoding are client-go's: tikv/kv.go GcSavedSafePoint,
	// stored as a decimal string.
	body, err := json.Marshal(map[string]string{
		"key":   base64.StdEncoding.EncodeToString([]byte("/tidb/store/gcworker/saved_safe_point")),
		"value": base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(safePoint, 10))),
	})
	if err != nil {
		return Error.Wrap(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v3/kv/put", bytes.NewReader(body))
	if err != nil {
		return Error.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return Error.New("unexpected status %d from etcd put", resp.StatusCode)
	}
	return nil
}

// waitGCEnforced blocks until TiDB rejects a read just below safePointTime,
// i.e. until the armed safe point is actually being enforced against reads.
func waitGCEnforced(ctx context.Context, db *sql.DB, table string, safePointTime time.Time) error {
	below := dbutil.TiDB.AsOfSystemTime(safePointTime.Add(-time.Millisecond))
	for {
		var id int64
		err := db.QueryRowContext(ctx, "SELECT id FROM "+table+below+"LIMIT 1").Scan(&id)
		if err != nil && strings.Contains(err.Error(), "Error 9006") {
			return nil
		}
		if err != nil && !errs.Is(err, sql.ErrNoRows) {
			return Error.New("probe read below the safe point failed unexpectedly: %w", err)
		}
		select {
		case <-ctx.Done():
			return Error.New("TiDB never enforced the GC safe point: a read below it still succeeds: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}
