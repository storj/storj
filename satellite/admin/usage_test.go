// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/orders"
)

// usageReportDateLayout is the time format expected by the generated admin handler for
// the since/before query parameters.
const usageReportDateLayout = "2006-01-02T15:04:05.999Z"

func TestGetUserUsageReport(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.UserGroupsRoleViewer = []string{"viewer"}
				config.Admin.BypassAuth = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		address := sat.Admin.Admin.Listener.Addr().String()
		baseURL := "http://" + address

		since := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		before := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)

		parseCSV := func(r io.Reader) [][]string {
			reader := csv.NewReader(r)
			records, err := reader.ReadAll()
			require.NoError(t, err)
			return records
		}

		hasColumn := func(header []string, name string) bool {
			for _, h := range header {
				if h == name {
					return true
				}
			}
			return false
		}

		columnIndex := func(header []string, name string) int {
			for i, h := range header {
				if h == name {
					return i
				}
			}
			return -1
		}

		// insertBucketData inserts attribution, 3 storage tallies (to get non-zero storage),
		// and an egress rollup for the given bucket.
		insertBucketData := func(projectID uuid.UUID, bucketName string) {
			_, err := sat.DB.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  projectID,
				BucketName: []byte(bucketName),
			})
			require.NoError(t, err)

			// 3 tallies: 2 contribute to storage-hours, 3rd acts as the fencepost.
			for i := 0; i < 3; i++ {
				tally := accounting.BucketStorageTally{
					BucketName:        bucketName,
					ProjectID:         projectID,
					IntervalStart:     since.Add(time.Duration(i) * time.Hour),
					TotalBytes:        1024 * 1024, // 1 MB
					ObjectCount:       10,
					TotalSegmentCount: 5,
				}
				require.NoError(t, sat.DB.ProjectAccounting().CreateStorageTally(ctx, tally))
			}

			require.NoError(t, sat.DB.Orders().UpdateBandwidthBatch(ctx, []orders.BucketBandwidthRollup{
				{
					ProjectID:     projectID,
					BucketName:    bucketName,
					IntervalStart: since.Add(time.Hour),
					Action:        pb.PieceAction_GET,
					Settled:       1024 * 1024 * 1024, // 1 GB
				},
			}))
		}

		newUser := func(email string) *console.User {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Test User",
				Email:    email,
			}, 10)
			require.NoError(t, err)
			return user
		}

		newProject := func(userID uuid.UUID, name string) *console.Project {
			proj, err := sat.AddProject(ctx, userID, name)
			require.NoError(t, err)
			return proj
		}

		// parseFloat is a shorthand for strconv.ParseFloat used in row data verification.
		parseFloat := func(s string) float64 {
			v, err := strconv.ParseFloat(s, 64)
			require.NoError(t, err)
			return v
		}

		// Expected numeric values per bucket (bucket-level mode, not project-summarized):
		//   storage (GB-hours): 2 × (1 MiB / 1e9) = 0.002097152
		//   egress (GB):        1 GiB / 1e9       = 1.073741824
		//   objectCount:        10 objects × 2 one-hour intervals = 20.0
		//   segmentCount:       5 segments × 2 one-hour intervals = 10.0
		const (
			storagePerBucket  = 0.002097152
			egressPerBucket   = 1.073741824
			objectsPerBucket  = 20.0
			segmentsPerBucket = 10.0
			// floatDelta accommodates up to 6-decimal-place CSV formatting of float values.
			floatDelta = 1e-5
		)

		// verifyBucketRow checks numeric and time fields for one bucket-level CSV data row.
		verifyBucketRow := func(header, row []string) {
			require.InDelta(t, storagePerBucket, parseFloat(row[columnIndex(header, "storage")]), floatDelta)
			require.InDelta(t, egressPerBucket, parseFloat(row[columnIndex(header, "egress")]), floatDelta)
			require.InDelta(t, objectsPerBucket, parseFloat(row[columnIndex(header, "objectCount")]), floatDelta)
			require.InDelta(t, segmentsPerBucket, parseFloat(row[columnIndex(header, "segmentCount")]), floatDelta)
			require.NotEmpty(t, row[columnIndex(header, "since")])
			require.NotEmpty(t, row[columnIndex(header, "before")])
		}

		t.Run("success all projects", func(t *testing.T) {
			user := newUser("all-projects@test.test")
			proj1 := newProject(user.ID, "Project 1")
			proj2 := newProject(user.ID, "Project 2")

			insertBucketData(proj1.ID, "bucket-p1-1")
			insertBucketData(proj1.ID, "bucket-p1-2")
			insertBucketData(proj2.ID, "bucket-p2-1")
			insertBucketData(proj2.ID, "bucket-p2-2")

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, since, before, uuid.UUID{}, false)
			require.NoError(t, apiErr.Err)

			records := parseCSV(w.Body)
			require.Len(t, records, 5) // 1 header + 4 data rows

			header := records[0]
			require.True(t, hasColumn(header, "projectName"))
			require.True(t, hasColumn(header, "projectPublicID"))
			require.True(t, hasColumn(header, "bucketName"))
			require.True(t, hasColumn(header, "storage"))
			require.True(t, hasColumn(header, "egress"))
			require.True(t, hasColumn(header, "objectCount"))
			require.True(t, hasColumn(header, "segmentCount"))
			require.True(t, hasColumn(header, "since"))
			require.True(t, hasColumn(header, "before"))
			require.False(t, hasColumn(header, "storageCost"))

			// Map each (projectPublicID, bucketName) to its expected project name.
			type rowKey struct{ pubID, bucket string }
			expectedRows := map[rowKey]string{
				{proj1.PublicID.String(), "bucket-p1-1"}: "Project 1",
				{proj1.PublicID.String(), "bucket-p1-2"}: "Project 1",
				{proj2.PublicID.String(), "bucket-p2-1"}: "Project 2",
				{proj2.PublicID.String(), "bucket-p2-2"}: "Project 2",
			}
			pubIDIdx := columnIndex(header, "projectPublicID")
			bucketIdx := columnIndex(header, "bucketName")
			nameIdx := columnIndex(header, "projectName")
			for _, row := range records[1:] {
				key := rowKey{row[pubIDIdx], row[bucketIdx]}
				expectedName, ok := expectedRows[key]
				require.True(t, ok, "unexpected row: pubID=%s bucket=%s", key.pubID, key.bucket)
				require.Equal(t, expectedName, row[nameIdx])
				verifyBucketRow(header, row)
			}
		})

		t.Run("success filter by projectID", func(t *testing.T) {
			user := newUser("filter-project@test.test")
			proj1 := newProject(user.ID, "Project Alpha")
			proj2 := newProject(user.ID, "Project Beta")

			insertBucketData(proj1.ID, "bucket-alpha-1")
			insertBucketData(proj1.ID, "bucket-alpha-2")
			insertBucketData(proj2.ID, "bucket-beta-1")
			insertBucketData(proj2.ID, "bucket-beta-2")

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, since, before, proj1.PublicID, false)
			require.NoError(t, apiErr.Err)

			records := parseCSV(w.Body)
			require.Len(t, records, 3) // 1 header + 2 data rows for proj1 only

			header := records[0]
			pubIDIdx := columnIndex(header, "projectPublicID")
			bucketIdx := columnIndex(header, "bucketName")
			require.GreaterOrEqual(t, pubIDIdx, 0)
			validBuckets := map[string]bool{"bucket-alpha-1": true, "bucket-alpha-2": true}
			for _, row := range records[1:] {
				require.Equal(t, proj1.PublicID.String(), row[pubIDIdx])
				require.Equal(t, "Project Alpha", row[columnIndex(header, "projectName")])
				require.True(t, validBuckets[row[bucketIdx]], "unexpected bucket: %s", row[bucketIdx])
				verifyBucketRow(header, row)
			}
		})

		t.Run("success no projects", func(t *testing.T) {
			user := newUser("no-projects@test.test")

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, since, before, uuid.UUID{}, false)
			require.NoError(t, apiErr.Err)

			records := parseCSV(w.Body)
			require.Len(t, records, 1) // header row only
		})

		t.Run("success no usage in period", func(t *testing.T) {
			user := newUser("no-usage@test.test")
			proj := newProject(user.ID, "Empty Project")

			// Insert attribution so the bucket exists, but no tallies in [since, before).
			_, err := sat.DB.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  proj.ID,
				BucketName: []byte("no-activity-bucket"),
			})
			require.NoError(t, err)

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, since, before, uuid.UUID{}, false)
			require.NoError(t, apiErr.Err)

			records := parseCSV(w.Body)
			require.Len(t, records, 1) // header row only
		})

		t.Run("success projectSummary=true", func(t *testing.T) {
			user := newUser("project-summary@test.test")
			proj1 := newProject(user.ID, "Summary Project 1")
			proj2 := newProject(user.ID, "Summary Project 2")

			insertBucketData(proj1.ID, "sum-p1-bucket1")
			insertBucketData(proj1.ID, "sum-p1-bucket2")
			insertBucketData(proj2.ID, "sum-p2-bucket1")
			insertBucketData(proj2.ID, "sum-p2-bucket2")

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, since, before, uuid.UUID{}, true)
			require.NoError(t, apiErr.Err)

			records := parseCSV(w.Body)
			require.Len(t, records, 3) // 1 header + 2 data rows (one per project)

			header := records[0]
			require.False(t, hasColumn(header, "bucketName"), "bucketName must not appear with projectSummary=true")
			require.True(t, hasColumn(header, "projectName"))
			require.True(t, hasColumn(header, "projectPublicID"))

			// With projectSummary=true, values are aggregated across 2 buckets per project.
			storagePerProject := 2 * storagePerBucket
			egressPerProject := 2 * egressPerBucket
			objectsPerProject := 2 * objectsPerBucket
			segmentsPerProject := 2 * segmentsPerBucket

			expectedProjects := map[string]string{
				proj1.PublicID.String(): "Summary Project 1",
				proj2.PublicID.String(): "Summary Project 2",
			}
			pubIDIdx := columnIndex(header, "projectPublicID")
			nameIdx := columnIndex(header, "projectName")
			for _, row := range records[1:] {
				pubID := row[pubIDIdx]
				expectedName, ok := expectedProjects[pubID]
				require.True(t, ok, "unexpected projectPublicID: %s", pubID)
				require.Equal(t, expectedName, row[nameIdx])
				require.InDelta(t, storagePerProject, parseFloat(row[columnIndex(header, "storage")]), floatDelta)
				require.InDelta(t, egressPerProject, parseFloat(row[columnIndex(header, "egress")]), floatDelta)
				require.InDelta(t, objectsPerProject, parseFloat(row[columnIndex(header, "objectCount")]), floatDelta)
				require.InDelta(t, segmentsPerProject, parseFloat(row[columnIndex(header, "segmentCount")]), floatDelta)
				require.NotEmpty(t, row[columnIndex(header, "since")])
				require.NotEmpty(t, row[columnIndex(header, "before")])
			}
		})

		t.Run("HTTP success response headers", func(t *testing.T) {
			user := newUser("headers@test.test")
			proj := newProject(user.ID, "Headers Project")
			insertBucketData(proj.ID, "headers-bucket")

			dateFormat := "2006-01-02"
			expectedFilename := "usage-report-" + user.ID.String() + "-" + since.Format(dateFormat) + "-to-" + before.Format(dateFormat) + ".csv"

			url := fmt.Sprintf(
				"%s/api/v1/users/%s/usage-report?since=%s&before=%s",
				baseURL, user.ID, since.Format(usageReportDateLayout), before.Format(usageReportDateLayout),
			)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "attachment; filename="+expectedFilename, resp.Header.Get("Content-Disposition"))
			require.Equal(t, "no-store, no-cache, must-revalidate, proxy-revalidate", resp.Header.Get("Cache-Control"))
			require.Equal(t, "no-cache", resp.Header.Get("Pragma"))
			require.Equal(t, "0", resp.Header.Get("Expires"))
		})

		t.Run("unknown projectID", func(t *testing.T) {
			user := newUser("unknown-proj-id@test.test")

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, since, before, testrand.UUID(), false)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("projectID belongs to another user", func(t *testing.T) {
			user1 := newUser("proj-owner-1@test.test")
			user2 := newUser("proj-owner-2@test.test")
			proj2 := newProject(user2.ID, "User 2 Project")

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user1.ID, since, before, proj2.PublicID, false)
			require.Equal(t, http.StatusForbidden, apiErr.Status)
		})

		t.Run("user not found", func(t *testing.T) {
			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, testrand.UUID(), since, before, uuid.UUID{}, false)
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("HTTP since malformed", func(t *testing.T) {
			user := newUser("bad-since@test.test")
			url := fmt.Sprintf(
				"%s/api/v1/users/%s/usage-report?since=not-a-time&before=%s&projectID=none&projectSummary=false",
				baseURL, user.ID, before.Format(usageReportDateLayout),
			)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("HTTP before malformed", func(t *testing.T) {
			user := newUser("bad-before@test.test")
			url := fmt.Sprintf(
				"%s/api/v1/users/%s/usage-report?since=%s&before=not-a-time&projectID=none&projectSummary=false",
				baseURL, user.ID, since.Format(usageReportDateLayout),
			)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})

		t.Run("since equal to before", func(t *testing.T) {
			user := newUser("since-eq-before@test.test")
			ts := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, ts, ts, uuid.UUID{}, false)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("since after before", func(t *testing.T) {
			user := newUser("since-after-before@test.test")
			laterTime := time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)
			earlierTime := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, user.ID, laterTime, earlierTime, uuid.UUID{}, false)
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("HTTP permission denied viewer", func(t *testing.T) {
			user := newUser("viewer-denied@test.test")

			// Enable auth and set allowed host so the host check passes but the
			// viewer role lacks PermAccountViewUsage.
			service.TestSetBypassAuth(false)
			service.TestSetAllowedHost(address)
			defer service.TestSetBypassAuth(true)

			url := fmt.Sprintf(
				"%s/api/v1/users/%s/usage-report?since=%s&before=%s",
				baseURL, user.ID, since.Format(usageReportDateLayout), before.Format(usageReportDateLayout),
			)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			require.NoError(t, err)
			req.Header.Set("X-Forwarded-Groups", "viewer")
			req.Header.Set("X-Forwarded-Email", "viewer@example.com")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})

		t.Run("isolation between users", func(t *testing.T) {
			userA := newUser("isolation-user-a@test.test")
			userB := newUser("isolation-user-b@test.test")
			projA := newProject(userA.ID, "User A Project")
			projB := newProject(userB.ID, "User B Project")

			insertBucketData(projA.ID, "isolation-bucket-a")
			insertBucketData(projB.ID, "isolation-bucket-b")

			w := httptest.NewRecorder()
			apiErr := service.GetUserUsageReport(ctx, w, userA.ID, since, before, uuid.UUID{}, false)
			require.NoError(t, apiErr.Err)

			records := parseCSV(w.Body)
			require.Len(t, records, 2) // 1 header + 1 data row for user A only

			header := records[0]
			pubIDIdx := columnIndex(header, "projectPublicID")
			require.GreaterOrEqual(t, pubIDIdx, 0)

			// Verify user A's row has correct values and user B's project is absent.
			for _, row := range records[1:] {
				require.NotEqual(t, projB.PublicID.String(), row[pubIDIdx])
			}
			row := records[1]
			require.Equal(t, projA.PublicID.String(), row[pubIDIdx])
			require.Equal(t, "User A Project", row[columnIndex(header, "projectName")])
			require.Equal(t, "isolation-bucket-a", row[columnIndex(header, "bucketName")])
			verifyBucketRow(header, row)
		})
	})
}
