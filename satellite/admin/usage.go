// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
)

// gb is the number of bytes in one gigabyte (SI definition, 1e9), used to
// convert byte-based accounting values to GB or GB-hours for the CSV output.
const gb = 1e9

// GetUserUsageReport generates a CSV report of storage and bandwidth usage for a user's projects.
func (s *Service) GetUserUsageReport(
	ctx context.Context,
	w http.ResponseWriter,
	userID uuid.UUID,
	since, before time.Time,
	projectID uuid.UUID, projectSummary bool,
) api.HTTPError {
	defer mon.Task()(&ctx)(nil)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{Status: status, Err: Error.Wrap(err)}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apiError(http.StatusNotFound, errs.New("user not found"))
		}
		return apiError(http.StatusInternalServerError, err)
	}
	if !s.userMatchesTenant(user.TenantID) {
		return apiError(http.StatusNotFound, errs.New("user not found"))
	}

	if !since.Before(before) {
		return apiError(http.StatusBadRequest, errs.New("since must be before before"))
	}

	type projectInfo struct {
		id       uuid.UUID
		publicID uuid.UUID
		name     string
	}

	var projects []projectInfo
	if projectID.IsZero() {
		consoleProjects, err := s.consoleDB.Projects().GetOwnActive(ctx, userID)
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}

		projects = make([]projectInfo, 0, len(consoleProjects))
		for _, p := range consoleProjects {
			projects = append(projects, projectInfo{id: p.ID, publicID: p.PublicID, name: p.Name})
		}
	} else {
		proj, err := s.consoleDB.Projects().GetByPublicID(ctx, projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apiError(http.StatusNotFound, errs.New("project not found"))
			}
			return apiError(http.StatusInternalServerError, err)
		}
		if proj.OwnerID != userID {
			return apiError(http.StatusForbidden, errs.New("project does not belong to user"))
		}

		projects = []projectInfo{{id: proj.ID, publicID: proj.PublicID, name: proj.Name}}
	}

	{
		filename := fmt.Sprintf(
			"usage-report-%s-%s-to-%s.csv",
			userID.String(), since.Format("2006-01-02"), before.Format("2006-01-02"),
		)
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}

	// CSV Writer is buffered through a bufio.Writer with its default size which is 4096 bytes.
	// The below returned errors may not be sent to the client because if an error writing or flushing
	// happens after a previous write which required flushing, therefore the client has already
	// received the HTTP 200 OK header and the above headers. The headers are sent before the first
	// byte of the HTTP body is sent.
	//
	// N.B. We could a in-memory buffer instead of writing to the response writer and then write the
	// whole data, however, that implies that we should establish a maximum buffer size—to mitigate
	// denial of service attacks—which entails that we may deny valid responses with a size bigger than
	// the buffer.
	csvWriter := csv.NewWriter(w)
	if projectSummary {
		header := []string{"projectName", "projectPublicID", "storage", "egress", "objectCount", "segmentCount", "since", "before"}
		if err := csvWriter.Write(header); err != nil {
			return apiError(http.StatusInternalServerError, err)
		}

		for _, proj := range projects {
			usages, err := s.accountingDB.GetProjectTotalByPlacement(ctx, proj.id, since, before, false)
			if err != nil {
				return apiError(http.StatusInternalServerError, err)
			}

			for _, usage := range usages {
				row := []string{
					proj.name,
					proj.publicID.String(),
					fmt.Sprintf("%f", usage.Storage/gb),
					fmt.Sprintf("%f", float64(usage.Egress)/gb),
					fmt.Sprintf("%f", usage.ObjectCount),
					fmt.Sprintf("%f", usage.SegmentCount),
					usage.Since.String(),
					usage.Before.String(),
				}
				if err := csvWriter.Write(row); err != nil {
					s.log.Error("error when writing a CSV row", zap.Error(err))
					return apiError(http.StatusInternalServerError, err)
				}
			}
		}
	} else {
		header := []string{"projectName", "projectPublicID", "bucketName", "storage", "egress", "objectCount", "segmentCount", "since", "before"}
		if err := csvWriter.Write(header); err != nil {
			return apiError(http.StatusInternalServerError, err)
		}

		for _, proj := range projects {
			rollups, err := s.accountingDB.GetBucketUsageRollups(ctx, proj.id, since, before, true)
			if err != nil {
				return apiError(http.StatusInternalServerError, err)
			}

			for _, rollup := range rollups {
				row := []string{
					proj.name,
					proj.publicID.String(),
					rollup.BucketName,
					fmt.Sprintf("%f", rollup.TotalStoredData),
					fmt.Sprintf("%f", rollup.GetEgress),
					fmt.Sprintf("%f", rollup.ObjectCount),
					fmt.Sprintf("%f", rollup.TotalSegments),
					rollup.Since.String(),
					rollup.Before.String(),
				}
				if err := csvWriter.Write(row); err != nil {
					s.log.Error("error when writing a CSV row", zap.Error(err))
					return apiError(http.StatusInternalServerError, err)
				}
			}
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		s.log.Error("error flushing the CSV writer", zap.Error(err))
		return apiError(http.StatusInternalServerError, err)
	}

	return api.HTTPError{}
}
