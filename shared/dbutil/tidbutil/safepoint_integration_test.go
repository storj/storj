// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	pd "github.com/tikv/pd/client"
	"github.com/tikv/pd/client/pkg/caller"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
)

// These tests run against a real PD+TiKV+TiDB cluster, which is expected to be
// reachable at STORJ_TEST_TIDB_PD, and skip without it. The TiDB the rest of the
// suite uses is a standalone server on the embedded unistore: fast, but it has no
// PD and therefore no real garbage collection, so it cannot serve these tests.
// testsuite/docker-compose.tidb-cluster.yaml brings up a real cluster separately,
// leaving that fast backend alone.
//
//	make test/tidb-cluster           # this file, minus the slow test
//	make test/tidb-cluster/gc-slow   # the slow test, on a fresh cluster
//
// Or by hand:
//
//	docker compose -f testsuite/docker-compose.tidb-cluster.yaml up -d
//	STORJ_TEST_TIDB_PD=127.0.0.1:2479 go test ./shared/dbutil/tidbutil/
//	docker compose -f testsuite/docker-compose.tidb-cluster.yaml down -v
//
// TestSafepointIntegration_GCWithheldAcrossCycles is opt-in (STORJ_TEST_TIDB_GC_SLOW=1)
// and takes ~25 minutes: TiDB refuses a GC life time under 10 minutes and offers no
// way to force a run, so waiting out two real cycles is the only way to watch GC
// actually being held back. It needs a fresh cluster, because proving a safepoint
// holds means making it the minimum, which permanently pushes gc_worker forward --
// PD allows neither lowering nor deleting it. The make targets recreate the cluster
// for you.
//
// On PD v8.5.x the GC barrier API does not exist, so Hold falls back to the
// legacy service GC safepoint API; that legacy path is what these tests cover.
//
// How PD actually withholds GC (verified empirically against v8.5.6, see
// TestSafepointIntegration_PDWithholdsGC): PD does not reject a service that
// asks to advance past another service's safepoint. Instead every
// UpdateServiceGCSafePoint call returns the *minimum* safepoint across all
// registered services, and TiDB's gc_worker is required to clamp its own GC run
// to that minimum. So "PD withholds GC" is observable as: while our hold is
// registered, the minimum PD hands back to everyone else is our TSO.

const (
	pdEndpointEnv = "STORJ_TEST_TIDB_PD"
	gcSlowEnv     = "STORJ_TEST_TIDB_GC_SLOW"
)

// serviceSafepoint is one entry of PD's /pd/api/v1/gc/safepoint response.
type serviceSafepoint struct {
	ServiceID string `json:"service_id"`
	ExpiredAt int64  `json:"expired_at"`
	SafePoint uint64 `json:"safe_point"`
}

// gcSafepointStatus is PD's /pd/api/v1/gc/safepoint response.
type gcSafepointStatus struct {
	ServiceGCSafePoints   []serviceSafepoint `json:"service_gc_safe_points"`
	MinServiceGCSafePoint uint64             `json:"min_service_gc_safe_point"`
	GCSafePoint           uint64             `json:"gc_safe_point"`
}

// find returns the entry for serviceID, if registered.
func (status gcSafepointStatus) find(serviceID string) (serviceSafepoint, bool) {
	for _, entry := range status.ServiceGCSafePoints {
		if entry.ServiceID == serviceID {
			return entry, true
		}
	}
	return serviceSafepoint{}, false
}

// pdHTTPBase turns a PD client endpoint ("127.0.0.1:2479") into an HTTP base URL.
func pdHTTPBase(endpoint string) string {
	first := strings.Split(endpoint, ",")[0]
	if strings.HasPrefix(first, "http://") || strings.HasPrefix(first, "https://") {
		return strings.TrimSuffix(first, "/")
	}
	return "http://" + first
}

// readGCSafepoints queries PD's HTTP API for the current GC safepoint state.
func readGCSafepoints(ctx context.Context, base string) (_ gcSafepointStatus, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/pd/api/v1/gc/safepoint", nil)
	if err != nil {
		return gcSafepointStatus{}, Error.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return gcSafepointStatus{}, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return gcSafepointStatus{}, Error.New("unexpected status %d from PD", resp.StatusCode)
	}

	var status gcSafepointStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return gcSafepointStatus{}, Error.Wrap(err)
	}
	return status, nil
}

// tsoWall converts a TSO to its wall clock time, dropping the logical bits.
func tsoWall(tso uint64) time.Time {
	return time.UnixMilli(int64(tso >> tsoPhysicalShiftBits)).UTC()
}

// requirePD skips the test unless a real PD endpoint is configured.
func requirePD(t *testing.T) string {
	endpoint := os.Getenv(pdEndpointEnv)
	if endpoint == "" {
		t.Skipf("%s is not set; start a cluster with "+
			"`docker compose -f testsuite/docker-compose.tidb-cluster.yaml up -d` "+
			"and set %s=127.0.0.1:2479", pdEndpointEnv, pdEndpointEnv)
	}
	return endpoint
}

// TestSafepointIntegration_PDWithholdsGC proves that a live PD keeps the
// effective GC safepoint pinned at the timestamp our Holder registered.
//
// Why this is not vacuous:
//   - We assert our own service_id is absent before Hold and after Release, and
//     present in between, so a no-op Hold fails at step 2.
//   - We do not assert only on the aggregate min: a leftover hold from an
//     earlier run could satisfy that without our barrier doing anything. We
//     locate our own entry by service_id.
//   - Crucially, on an idle cluster gc_worker sits ~10 minutes *behind* now, so
//     it is always the minimum and would mask our hold entirely: `min <= ours`
//     would hold whether or not we registered anything. To make our hold the
//     *binding* minimum we first push every other service safepoint past ours,
//     then assert PD hands back exactly our TSO.
func TestSafepointIntegration_PDWithholdsGC(t *testing.T) {
	endpoint := requirePD(t)
	base := pdHTTPBase(endpoint)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const serviceIDPrefix = "storj-gc-integration"

	// (6) Meaningfulness: nothing of ours is registered up front. Hold must be
	// what creates the entry.
	before, err := readGCSafepoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range before.ServiceGCSafePoints {
		if strings.HasPrefix(entry.ServiceID, serviceIDPrefix) {
			t.Fatalf("a %q safepoint is already registered (%s); "+
				"a leftover hold would make this test vacuous", entry.ServiceID, entry.ServiceID)
		}
	}

	// (1) Take the hold.
	holder, err := Hold(ctx, zaptest.NewLogger(t), SafepointConfig{
		PDEndpoints: endpoint,
		ServiceID:   serviceIDPrefix,
		TTL:         10 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			_ = holder.Release(ctx)
		}
	}()

	// (2) Our own entry exists, and PD protects exactly the timestamp we report
	// to callers as safe to read at.
	during, err := readGCSafepoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	ours, ok := during.find(holder.ServiceID())
	if !ok {
		t.Fatalf("no service safepoint registered for %q; got %+v",
			holder.ServiceID(), during.ServiceGCSafePoints)
	}
	// ReadTime drops the logical bits, so compare at millisecond precision
	// rather than against the raw TSO.
	if got, expected := tsoWall(ours.SafePoint), holder.ReadTime(); !got.Equal(expected) {
		t.Fatalf("PD protects %v (tso %d) but ReadTime is %v", got, ours.SafePoint, expected)
	}

	// (3) Our hold is at or ahead of the effective minimum, i.e. PD is not
	// already collecting past the snapshot we intend to read.
	if during.MinServiceGCSafePoint > ours.SafePoint {
		t.Fatalf("min_service_gc_safe_point %d is ahead of our safepoint %d",
			during.MinServiceGCSafePoint, ours.SafePoint)
	}

	// (4) PD enforces the hold.
	client, err := pd.NewClientWithContext(ctx, caller.Component("storj/gc-safepoint-test"),
		strings.Split(endpoint, ","), pd.SecurityOption{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Something well past our hold, which no honest GC run may collect up to
	// while we hold.
	ahead := ours.SafePoint + uint64(time.Hour/time.Millisecond)<<tsoPhysicalShiftBits

	// (4a) A competing ordinary service asks to advance past us. PD accepts the
	// registration but reports back the minimum, which cannot exceed our hold.
	competitorID := serviceIDPrefix + "-competitor"
	//lint:ignore SA1019 the barrier API that replaces this does not exist on PD v8.5.x, which is what Hold falls back from.
	//nolint:staticcheck // the barrier API that replaces this does not exist on PD v8.5.x, which is what Hold falls back from.
	competitorMin, err := client.UpdateServiceGCSafePoint(ctx, competitorID, 600, ahead)
	if err != nil {
		t.Fatal(err)
	}
	// ttl<=0 removes the entry again.
	defer func() {
		//lint:ignore SA1019 deprecated is fine here; see above.
		//nolint:staticcheck // deprecated is fine here; see above.
		if _, err := client.UpdateServiceGCSafePoint(ctx, competitorID, 0, ahead); err != nil {
			t.Errorf("removing competitor safepoint: %v", err)
		}
	}()
	if competitorMin > ours.SafePoint {
		t.Fatalf("PD let the minimum advance to %d past our hold at %d",
			competitorMin, ours.SafePoint)
	}

	// (4b) The real test: make our hold the binding minimum. gc_worker is the
	// service TiDB's GC actually runs as, and on an idle cluster it sits behind
	// us, masking our hold. Push it just past our hold -- exactly the situation
	// our Holder exists to defend against -- and PD must hand back *our* TSO as
	// the ceiling for the GC run.
	//
	// PD requires gc_worker's TTL to be infinite ("TTL of gc_worker's service
	// safe point must be infinity"), and gc_worker's entry can be neither
	// lowered nor removed, so this is deliberately the smallest possible push
	// (one logical tick past our hold) to limit the damage to the cluster.
	//lint:ignore SA1019 deprecated is fine here; see above.
	//nolint:staticcheck // deprecated is fine here; see above.
	if _, err := client.UpdateServiceGCSafePoint(ctx, "gc_worker", math.MaxInt64, ours.SafePoint+1); err != nil {
		t.Fatal(err)
	}

	// With every other service at or beyond our hold, the minimum PD reports is
	// ours and nothing else. Were the hold absent, this would be ahead.
	//lint:ignore SA1019 deprecated is fine here; see above.
	//nolint:staticcheck // deprecated is fine here; see above.
	blockedMin, err := client.UpdateServiceGCSafePoint(ctx, competitorID, 600, ahead)
	if err != nil {
		t.Fatal(err)
	}
	if blockedMin != ours.SafePoint {
		t.Fatalf("PD reported minimum %d (%v), expected our held safepoint %d (%v)",
			blockedMin, tsoWall(blockedMin), ours.SafePoint, tsoWall(ours.SafePoint))
	}

	held, err := readGCSafepoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if held.MinServiceGCSafePoint != ours.SafePoint {
		t.Fatalf("min_service_gc_safe_point %d, expected our held safepoint %d",
			held.MinServiceGCSafePoint, ours.SafePoint)
	}
	if held.GCSafePoint > ours.SafePoint {
		t.Fatalf("cluster gc_safe_point %d advanced past our hold at %d",
			held.GCSafePoint, ours.SafePoint)
	}

	// (5) Release removes our entry from PD.
	released = true
	if err := holder.Release(ctx); err != nil {
		t.Fatal(err)
	}

	after, err := readGCSafepoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if entry, ok := after.find(holder.ServiceID()); ok {
		t.Fatalf("safepoint %q still registered after release: %+v", holder.ServiceID(), entry)
	}

	// ...and with it gone, the minimum is free to move past where we held.
	//lint:ignore SA1019 deprecated is fine here; see above.
	//nolint:staticcheck // deprecated is fine here; see above.
	freedMin, err := client.UpdateServiceGCSafePoint(ctx, competitorID, 600, ahead)
	if err != nil {
		t.Fatal(err)
	}
	if freedMin <= ours.SafePoint {
		t.Fatalf("minimum %d did not advance past the released hold at %d; "+
			"the hold was not what was blocking it", freedMin, ours.SafePoint)
	}
}

// TestSafepointIntegration_GCWithheldAcrossCycles watches a live TiDB GC worker
// across two GC cycles and asserts it never collects past a live hold, then
// that it does once the hold is released.
//
// Unlike the fast test this does not simulate gc_worker; it waits for the real
// one. GC cannot be sped up (validateGCLifeTime clamps tikv_gc_life_time below
// 10m and there is no force-run), hence the runtime.
//
// This test needs a cluster whose gc_worker has *not* been pushed forward by
// TestSafepointIntegration_PDWithholdsGC -- gc_worker's safepoint cannot be
// lowered or removed. Run it against a fresh cluster:
//
//	docker compose -f testsuite/docker-compose.tidb-cluster.yaml down -v
//	docker compose -f testsuite/docker-compose.tidb-cluster.yaml up -d
func TestSafepointIntegration_GCWithheldAcrossCycles(t *testing.T) {
	endpoint := requirePD(t)
	if os.Getenv(gcSlowEnv) != "1" {
		t.Skipf("set %s=1 to run; it waits out two real GC cycles (~25 min) "+
			"and needs -timeout 60m", gcSlowEnv)
	}
	base := pdHTTPBase(endpoint)

	// Not testcontext.New: its default deadline is 3 minutes, which this test
	// blows through long before the first GC cycle lands.
	ctx := testcontext.NewWithTimeout(t, 45*time.Minute)
	defer ctx.Cleanup()

	holder, err := Hold(ctx, zaptest.NewLogger(t), SafepointConfig{
		PDEndpoints: endpoint,
		// deliberately not prefixed with the fast test's serviceIDPrefix, whose
		// leftover check would otherwise trip over this test's hold
		ServiceID: "storj-gc-slow",
		TTL:       10 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			_ = holder.Release(ctx)
		}
	}()

	status, err := readGCSafepoints(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	ours, ok := status.find(holder.ServiceID())
	if !ok {
		t.Fatalf("no service safepoint registered for %q", holder.ServiceID())
	}
	t.Logf("holding %d (%v)", ours.SafePoint, tsoWall(ours.SafePoint))

	gcWorkerStart, ok := status.find("gc_worker")
	if !ok {
		t.Fatal("no gc_worker safepoint; is TiDB running against this PD?")
	}

	// Two GC cycles, polling every 30s.
	const holdFor = 21 * time.Minute
	gcWorkerLatest := gcWorkerStart
	deadline := time.Now().Add(holdFor)
	for time.Now().Before(deadline) {
		if !sleepCtx(ctx, 30*time.Second) {
			t.Fatal(ctx.Err())
		}

		status, err := readGCSafepoints(ctx, base)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := status.find(holder.ServiceID()); !ok {
			t.Fatalf("our hold %q disappeared from PD while still held", holder.ServiceID())
		}

		// The hold is alive: real GC must not have collected past it.
		if status.GCSafePoint > ours.SafePoint {
			t.Fatalf("cluster gc_safe_point advanced to %d (%v) past our hold at %d (%v)",
				status.GCSafePoint, tsoWall(status.GCSafePoint),
				ours.SafePoint, tsoWall(ours.SafePoint))
		}
		if status.MinServiceGCSafePoint > ours.SafePoint {
			t.Fatalf("min_service_gc_safe_point advanced to %d past our hold at %d",
				status.MinServiceGCSafePoint, ours.SafePoint)
		}

		if entry, ok := status.find("gc_worker"); ok && entry.SafePoint > gcWorkerLatest.SafePoint {
			gcWorkerLatest = entry
			t.Logf("gc_worker advanced to %d (%v); gc_safe_point=%d (%v)",
				entry.SafePoint, tsoWall(entry.SafePoint),
				status.GCSafePoint, tsoWall(status.GCSafePoint))
		}
	}

	// Rules out the vacuous pass where GC simply never ran: if gc_worker never
	// moved, the assertions above proved nothing.
	if gcWorkerLatest.SafePoint <= gcWorkerStart.SafePoint {
		t.Fatalf("gc_worker never advanced from %d (%v) in %v: GC is not running, "+
			"so withholding was never actually exercised",
			gcWorkerStart.SafePoint, tsoWall(gcWorkerStart.SafePoint), holdFor)
	}
	t.Logf("gc_worker advanced %d -> %d while the hold blocked collection past %d",
		gcWorkerStart.SafePoint, gcWorkerLatest.SafePoint, ours.SafePoint)

	// Release and let one more GC cycle run: collection must now move past
	// where we were holding.
	released = true
	if err := holder.Release(ctx); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(13 * time.Minute)
	for time.Now().Before(deadline) {
		if !sleepCtx(ctx, 30*time.Second) {
			t.Fatal(ctx.Err())
		}

		status, err := readGCSafepoints(ctx, base)
		if err != nil {
			t.Fatal(err)
		}
		if status.GCSafePoint > ours.SafePoint {
			t.Logf("after release gc_safe_point advanced to %d (%v), past the former hold at %d (%v)",
				status.GCSafePoint, tsoWall(status.GCSafePoint),
				ours.SafePoint, tsoWall(ours.SafePoint))
			return
		}
	}
	t.Fatalf("cluster gc_safe_point never advanced past the released hold at %d (%v)",
		ours.SafePoint, tsoWall(ours.SafePoint))
}

// sleepCtx sleeps for d, reporting false if ctx ended first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
