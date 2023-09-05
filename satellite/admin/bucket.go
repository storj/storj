// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
)

func validateBucketPathParameters(vars map[string]string) (project uuid.NullUUID, bucket []byte, err error) {
	projectUUIDString, ok := vars["project"]
	if !ok {
		return project, bucket, fmt.Errorf("project-uuid missing")
	}

	project.UUID, err = uuidFromString(projectUUIDString)
	if err != nil {
		return project, bucket, fmt.Errorf("project-uuid is not a valid uuid")
	}
	project.Valid = true

	bucketName := vars["bucket"]
	if len(bucketName) == 0 {
		return project, bucket, fmt.Errorf("bucket name is missing")
	}

	bucket = []byte(bucketName)
	return
}

func parsePlacementConstraint(regionCode string) (storj.PlacementConstraint, error) {
	switch regionCode {
	case "EU":
		return storj.EU, nil
	case "EEA":
		return storj.EEA, nil
	case "US":
		return storj.US, nil
	case "DE":
		return storj.DE, nil
	case "NR":
		return storj.NR, nil
	case "":
		return storj.EveryCountry, fmt.Errorf("missing region parameter")
	default:
		return storj.EveryCountry, fmt.Errorf("unrecognized region parameter: %s", regionCode)
	}
}

func (server *Server) updateBucket(w http.ResponseWriter, r *http.Request, placement storj.PlacementConstraint) {
	ctx := r.Context()

	project, bucket, err := validateBucketPathParameters(mux.Vars(r))
	if err != nil {
		sendJSONError(w, err.Error(), "", http.StatusBadRequest)
		return
	}

	b, err := server.buckets.GetBucket(ctx, bucket, project.UUID)
	if err != nil {
		if buckets.ErrBucketNotFound.Has(err) {
			sendJSONError(w, "bucket does not exist", "", http.StatusBadRequest)
		} else {
			sendJSONError(w, "unable to create geofence for bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	b.Placement = placement

	_, err = server.buckets.UpdateBucket(ctx, b)
	if err != nil {
		switch {
		case buckets.ErrBucketNotFound.Has(err):
			sendJSONError(w, "bucket does not exist", "", http.StatusBadRequest)
		case buckets.ErrBucketNotEmpty.Has(err):
			sendJSONError(w, "bucket must be empty", "", http.StatusBadRequest)
		default:
			sendJSONError(w, "unable to create geofence for bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (server *Server) createGeofenceForBucket(w http.ResponseWriter, r *http.Request) {
	placement, err := parsePlacementConstraint(r.URL.Query().Get("region"))
	if err != nil {
		sendJSONError(w, err.Error(), "available: EU, EEA, US, DE, NR", http.StatusBadRequest)
		return
	}

	server.updateBucket(w, r, placement)
}

func (server *Server) deleteGeofenceForBucket(w http.ResponseWriter, r *http.Request) {
	server.updateBucket(w, r, storj.EveryCountry)
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
