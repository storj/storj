// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
)

func validateBucketPathParameters(vars map[string]string) (project uuid.NullUUID, bucket []byte, err error) {
	projectUUIDString, ok := vars["project"]
	if !ok {
		return project, bucket, errors.New("project-uuid missing")
	}

	project.UUID, err = uuidFromString(projectUUIDString)
	if err != nil {
		return project, bucket, errors.New("project-uuid is not a valid uuid")
	}
	project.Valid = true

	bucketName := vars["bucket"]
	if len(bucketName) == 0 {
		return project, bucket, errors.New("bucket name is missing")
	}

	bucket = []byte(bucketName)
	return
}

func (server *Server) updatePlacementForBucket(w http.ResponseWriter, r *http.Request) {
	placementID := r.URL.Query().Get("id")
	if placementID == "" {
		sendJSONError(w, "missing id parameter", "", http.StatusBadRequest)
		return
	}

	parsed, err := strconv.ParseUint(placementID, 0, 16)
	if err != nil {
		sendJSONError(w, "invalid placement parameter", err.Error(), http.StatusBadRequest)
		return
	}

	placement := storj.PlacementConstraint(parsed)

	if _, ok := server.placement[placement]; !ok {
		sendJSONError(w, "unknown placement parameter", "", http.StatusBadRequest)
		return
	}

	server.updateBucket(w, r, placement)
}

func (server *Server) updateBucket(w http.ResponseWriter, r *http.Request, placement storj.PlacementConstraint) {
	project, bucket, err := validateBucketPathParameters(mux.Vars(r))
	if err != nil {
		sendJSONError(w, err.Error(), "", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	b, err := server.buckets.GetBucket(ctx, bucket, project.UUID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			sendJSONError(w, "bucket does not exist", "", http.StatusNotFound)
		} else {
			sendJSONError(w, "unable to update placement for bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	_, err = server.db.Attribution().Get(ctx, project.UUID, bucket)
	if err != nil {
		if attribution.ErrBucketNotAttributed.Has(err) {
			sendJSONError(w, "bucket attribution does not exist", "", http.StatusNotFound)
		} else {
			sendJSONError(w, "unable to update placement for bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	b.Placement = placement

	_, err = server.buckets.UpdateBucket(ctx, b)
	if err != nil {
		switch {
		case buckets.ErrBucketNotFound.Has(err):
			sendJSONError(w, "bucket does not exist", "", http.StatusNotFound)
		case buckets.ErrBucketNotEmpty.Has(err):
			sendJSONError(w, "bucket must be empty", "", http.StatusBadRequest)
		default:
			sendJSONError(w, "unable to create geofence for bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (server *Server) getBucketInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	project, bucket, err := validateBucketPathParameters(mux.Vars(r))
	if err != nil {
		sendJSONError(w, err.Error(), "", http.StatusBadRequest)
		return
	}

	b, err := server.buckets.GetBucket(ctx, bucket, project.UUID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			sendJSONError(w, "bucket does not exist", "", http.StatusNotFound)
		} else {
			sendJSONError(w, "unable to check bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	data, err := json.Marshal(b)
	if err != nil {
		sendJSONError(w, "failed to marshal bucket", err.Error(), http.StatusInternalServerError)
	} else {
		sendJSONData(w, http.StatusOK, data)
	}
}

func (server *Server) updateBucketValueAttributionPlacement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	project, bucket, err := validateBucketPathParameters(mux.Vars(r))
	if err != nil {
		sendJSONError(w, err.Error(), "", http.StatusBadRequest)
		return
	}

	var newPlacement *storj.PlacementConstraint
	placementStr := strings.ToUpper(r.URL.Query().Get("placement"))

	switch {
	case placementStr == "":
		sendJSONError(w, "missing placement parameter", "", http.StatusBadRequest)
		return
	case placementStr == "NULL":
		newPlacement = nil
	default:
		parsed, err := strconv.ParseUint(placementStr, 0, 16)
		if err != nil {
			sendJSONError(w, "invalid placement parameter", err.Error(), http.StatusBadRequest)
			return
		}

		placementVal := storj.PlacementConstraint(parsed)
		if _, ok := server.placement[placementVal]; !ok {
			sendJSONError(w, "unknown placement parameter", "", http.StatusBadRequest)
			return
		}

		newPlacement = &placementVal
	}

	_, err = server.db.Attribution().Get(ctx, project.UUID, bucket)
	if err != nil {
		if attribution.ErrBucketNotAttributed.Has(err) {
			sendJSONError(w, "bucket attribution does not exist", "", http.StatusNotFound)
		} else {
			sendJSONError(w, "unable to get placement for a bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	err = server.db.Attribution().UpdatePlacement(ctx, project.UUID, string(bucket), newPlacement)
	if err != nil {
		sendJSONError(w, "unable to update placement for a bucket", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
