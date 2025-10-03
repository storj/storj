// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/zeebo/errs/v2"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/payments/stripe"
)

func (server *Server) checkProjectUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "error getting project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	if !server.checkUsage(ctx, w, project.ID) {
		sendJSONData(w, http.StatusOK, []byte(`{"result":"no project usage exist"}`))
	}
}

func (server *Server) getProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		sendJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "unable to fetch project details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(project)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) getProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var output struct {
		Usage struct {
			Amount memory.Size `json:"amount"`
			Bytes  int64       `json:"bytes"`
		} `json:"usage"`
		Bandwidth struct {
			Amount memory.Size `json:"amount"`
			Bytes  int64       `json:"bytes"`
		} `json:"bandwidth"`
		Buckets     *int  `json:"maxBuckets"`
		Segments    int64 `json:"maxSegments"`
		Rate        *int  `json:"rate"`
		Burst       *int  `json:"burst"`
		RateHead    *int  `json:"rateHead"`
		BurstHead   *int  `json:"burstHead"`
		RateGet     *int  `json:"rateGet"`
		BurstGet    *int  `json:"burstGet"`
		RatePut     *int  `json:"ratePut"`
		BurstPut    *int  `json:"burstPut"`
		RateList    *int  `json:"rateList"`
		BurstList   *int  `json:"burstList"`
		RateDelete  *int  `json:"rateDelete"`
		BurstDelete *int  `json:"burstDelete"`
	}
	if project.StorageLimit != nil {
		output.Usage.Amount = *project.StorageLimit
		output.Usage.Bytes = project.StorageLimit.Int64()
	}
	if project.BandwidthLimit != nil {
		output.Bandwidth.Amount = *project.BandwidthLimit
		output.Bandwidth.Bytes = project.BandwidthLimit.Int64()
	}

	output.Buckets = project.MaxBuckets

	if project.SegmentLimit != nil {
		output.Segments = *project.SegmentLimit
	}

	output.Rate = project.RateLimit
	output.Burst = project.BurstLimit
	output.RateHead = project.RateLimitHead
	output.BurstHead = project.BurstLimitHead
	output.RateGet = project.RateLimitGet
	output.BurstGet = project.BurstLimitGet
	output.RatePut = project.RateLimitPut
	output.BurstPut = project.BurstLimitPut
	output.RateList = project.RateLimitList
	output.BurstList = project.BurstLimitList
	output.RateDelete = project.RateLimitDelete
	output.BurstDelete = project.BurstLimitDelete

	data, err := json.Marshal(output)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) putProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	var arguments struct {
		Usage       *memory.Size `schema:"usage"`
		Bandwidth   *memory.Size `schema:"bandwidth"`
		Buckets     *int         `schema:"buckets"`
		Segments    *int64       `schema:"segments"`
		Rate        *int         `schema:"rate"`
		Burst       *int         `schema:"burst"`
		RateHead    *int         `schema:"rateHead"`
		BurstHead   *int         `schema:"burstHead"`
		RateGet     *int         `schema:"rateGet"`
		BurstGet    *int         `schema:"burstGet"`
		RatePut     *int         `schema:"ratePut"`
		BurstPut    *int         `schema:"burstPut"`
		RateList    *int         `schema:"rateList"`
		BurstList   *int         `schema:"burstList"`
		RateDelete  *int         `schema:"rateDelete"`
		BurstDelete *int         `schema:"burstDelete"`
	}

	if err := r.ParseForm(); err != nil {
		sendJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	decoder := schema.NewDecoder()
	err := decoder.Decode(&arguments, r.Form)
	if err != nil {
		sendJSONError(w, "invalid arguments",
			err.Error(), http.StatusBadRequest)
		return
	}

	// check if the project exists.
	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	toUpdate := []console.Limit{}
	if arguments.Usage != nil {
		if *arguments.Usage < 0 {
			sendJSONError(w, "negative usage",
				fmt.Sprintf("%v", arguments.Usage), http.StatusBadRequest)
			return
		}

		val := arguments.Usage.Int64()
		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.StorageLimit,
			Value: &val,
		})
	}

	if arguments.Bandwidth != nil {
		if *arguments.Bandwidth < 0 {
			sendJSONError(w, "negative bandwidth",
				fmt.Sprintf("%v", arguments.Usage), http.StatusBadRequest)
			return
		}

		val := arguments.Bandwidth.Int64()
		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BandwidthLimit,
			Value: &val,
		})
	}

	if arguments.Buckets != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.Buckets >= 0 {
			newVal := int64(*arguments.Buckets)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BucketsLimit,
			Value: val,
		})
	}

	if arguments.Segments != nil {
		if *arguments.Segments < 0 {
			sendJSONError(w, "negative segments count",
				fmt.Sprintf("t: %v", arguments.Buckets), http.StatusBadRequest)
			return
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.SegmentLimit,
			Value: arguments.Segments,
		})
	}

	if arguments.Rate != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.Rate >= 0 {
			newVal := int64(*arguments.Rate)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.RateLimit,
			Value: val,
		})
	}

	if arguments.Burst != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.Burst >= 0 {
			newVal := int64(*arguments.Burst)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BurstLimit,
			Value: val,
		})
	}

	if arguments.RateHead != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.RateHead >= 0 {
			newVal := int64(*arguments.RateHead)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.RateLimitHead,
			Value: val,
		})
	}

	if arguments.BurstHead != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.BurstHead >= 0 {
			newVal := int64(*arguments.BurstHead)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BurstLimitHead,
			Value: val,
		})
	}

	if arguments.RateGet != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.RateGet >= 0 {
			newVal := int64(*arguments.RateGet)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.RateLimitGet,
			Value: val,
		})
	}

	if arguments.BurstGet != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.BurstGet >= 0 {
			newVal := int64(*arguments.BurstGet)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BurstLimitGet,
			Value: val,
		})
	}

	if arguments.RatePut != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.RatePut >= 0 {
			newVal := int64(*arguments.RatePut)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.RateLimitPut,
			Value: val,
		})
	}

	if arguments.BurstPut != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.BurstPut >= 0 {
			newVal := int64(*arguments.BurstPut)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BurstLimitPut,
			Value: val,
		})
	}

	if arguments.RateList != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.RateList >= 0 {
			newVal := int64(*arguments.RateList)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.RateLimitList,
			Value: val,
		})
	}

	if arguments.BurstList != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.BurstList >= 0 {
			newVal := int64(*arguments.BurstList)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BurstLimitList,
			Value: val,
		})
	}

	if arguments.RateDelete != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.RateDelete >= 0 {
			newVal := int64(*arguments.RateDelete)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.RateLimitDelete,
			Value: val,
		})
	}

	if arguments.BurstDelete != nil {
		// Receiving a negative number means to apply defaults, which is indicated in the DB with null.
		var val *int64
		if *arguments.BurstDelete >= 0 {
			newVal := int64(*arguments.BurstDelete)
			val = &newVal
		}

		toUpdate = append(toUpdate, console.Limit{
			Kind:  console.BurstLimitDelete,
			Value: val,
		})
	}

	err = server.db.Console().Projects().UpdateLimitsGeneric(ctx, project.ID, toUpdate)
	if err != nil {
		sendJSONError(w, "failed to update usage",
			err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) addProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		OwnerID     uuid.UUID `json:"ownerId"`
		ProjectName string    `json:"projectName"`
	}

	var output struct {
		ProjectID uuid.UUID `json:"projectId"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.OwnerID.IsZero() {
		sendJSONError(w, "OwnerID is not set",
			"", http.StatusBadRequest)
		return
	}

	if input.ProjectName == "" {
		sendJSONError(w, "ProjectName is not set",
			"", http.StatusBadRequest)
		return
	}

	p := console.Project{
		Name:    input.ProjectName,
		OwnerID: input.OwnerID,
	}
	if server.entitlementsCfg.Enabled && len(server.console.Placement.AllowedPlacementIdsForNewProjects) > 0 {
		p.DefaultPlacement = server.console.Placement.AllowedPlacementIdsForNewProjects[0]
	}
	err = server.db.Console().WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		project, err := tx.Projects().Insert(ctx, &p)
		if err != nil {
			return err
		}

		_, err = tx.ProjectMembers().Insert(ctx, project.OwnerID, project.ID, console.RoleAdmin)
		if err != nil {
			return err
		}

		if server.entitlementsCfg.Enabled {
			// We have to use a direct DB call here because we are in a transaction.
			feats := entitlements.ProjectFeatures{NewBucketPlacements: server.console.Placement.AllowedPlacementIdsForNewProjects}
			featBytes, err := json.Marshal(feats)
			if err != nil {
				return err
			}

			_, err = tx.Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
				Scope:     entitlements.ConvertPublicIDToProjectScope(project.PublicID),
				Features:  featBytes,
				UpdatedAt: server.nowFn(),
			})
			if err != nil {
				return err
			}
		}

		output.ProjectID = project.ID

		return nil
	})
	if err != nil {
		sendJSONError(w, "failed to create project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(output)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) renameProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "error getting project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "ailed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		ProjectName string `json:"projectName"`
		Description string `json:"description"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.ProjectName == "" {
		sendJSONError(w, "ProjectName is not set",
			"", http.StatusBadRequest)
		return
	}

	project.Name = input.ProjectName
	if input.Description != "" {
		project.Description = input.Description
	}

	err = server.db.Console().Projects().Update(ctx, project)
	if err != nil {
		sendJSONError(w, "error renaming project",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) updateProjectsUserAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "error getting project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	creationDatePlusMonth := project.CreatedAt.AddDate(0, 1, 0)
	if time.Now().After(creationDatePlusMonth) {
		sendJSONError(w, "this project was created more than a month ago",
			"we should update user agent only for recently created projects", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		UserAgent string `json:"userAgent"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.UserAgent == "" {
		sendJSONError(w, "UserAgent was not provided",
			"", http.StatusBadRequest)
		return
	}

	newUserAgent := []byte(input.UserAgent)

	if bytes.Equal(project.UserAgent, newUserAgent) {
		sendJSONError(w, "new UserAgent is equal to existing projects UserAgent",
			"", http.StatusBadRequest)
		return
	}

	err = server._updateProjectsUserAgent(ctx, project.ID, newUserAgent)
	if err != nil {
		sendJSONError(w, "failed to update projects user agent",
			err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) updateComputeAccessToken(w http.ResponseWriter, r *http.Request) {
	if !server.entitlementsCfg.Enabled {
		sendJSONError(w, "entitlements are disabled", "", http.StatusForbidden)
		return
	}

	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "error getting project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		AccessToken *string `json:"accessToken"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	var newAccessToken []byte
	if input.AccessToken != nil {
		if *input.AccessToken == "" {
			sendJSONError(w, "new token was not provided",
				"", http.StatusBadRequest)
			return
		}
		newAccessToken = []byte(*input.AccessToken)
	}

	feats, err := server.entitlements.Projects().GetByPublicID(ctx, project.PublicID)
	if err != nil {
		if entitlements.ErrNotFound.Has(err) {
			feats = entitlements.ProjectFeatures{}
		} else {
			sendJSONError(w, "failed to get project entitlements",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if bytes.Equal(feats.ComputeAccessToken, newAccessToken) {
		sendJSONError(w, "new token is equal to existing ComputeAccessToken",
			"", http.StatusBadRequest)
		return
	}

	err = server.entitlements.Projects().SetComputeAccessTokenByPublicID(ctx, project.PublicID, newAccessToken)
	if err != nil {
		sendJSONError(w, "failed to update compute access token",
			err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if err != nil {
		sendJSONError(w, "error getting project",
			err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		sendJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().Get(ctx, project.OwnerID)
	if err != nil {
		sendJSONError(w, "error getting project owner",
			err.Error(), http.StatusBadRequest)
		return
	}

	if server.console.SelfServeAccountDeleteEnabled && user.Status == console.UserRequestedDeletion && (user.IsFree() || user.FinalInvoiceGenerated) {
		err = server.forceDeleteProject(ctx, project.ID)
		if err != nil {
			sendJSONError(w, "unable to delete project",
				err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	options := buckets.ListOptions{Limit: 1, Direction: buckets.DirectionForward}
	bucketsList, err := server.buckets.ListBuckets(ctx, project.ID, options, macaroon.AllowedBuckets{All: true})
	if err != nil {
		sendJSONError(w, "unable to list buckets",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if len(bucketsList.Items) > 0 {
		sendJSONError(w, "buckets still exist",
			fmt.Sprintf("%v", bucketNames(bucketsList.Items)), http.StatusConflict)
		return
	}

	// if usage exist, return error to client and exit
	if server.checkUsage(ctx, w, project.ID) {
		return
	}

	err = server.db.Console().APIKeys().DeleteAllByProjectID(ctx, project.ID)
	if err != nil {
		sendJSONError(w, "unable to delete api-keys",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.db.Console().Domains().DeleteAllByProjectID(ctx, project.ID)
	if err != nil {
		server.log.Error("failed to delete all domains for project",
			zap.String("project_id", project.ID.String()),
			zap.Error(err),
		)
	}

	// We update status to disabled instead of deleting the project
	// to not lose the historical project/user usage data.
	err = server.db.Console().Projects().UpdateStatus(ctx, project.ID, console.ProjectDisabled)
	if err != nil {
		sendJSONError(w, "unable to delete project",
			err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) forceDeleteProject(ctx context.Context, projectID uuid.UUID) error {
	listOptions := buckets.ListOptions{Direction: buckets.DirectionForward}
	allowedBuckets := macaroon.AllowedBuckets{All: true}

	bucketsList, err := server.buckets.ListBuckets(ctx, projectID, listOptions, allowedBuckets)
	if err != nil {
		return err
	}

	if len(bucketsList.Items) > 0 {
		var errList errs.Group
		for _, bucket := range bucketsList.Items {
			bucketLocation := metabase.BucketLocation{ProjectID: projectID, BucketName: metabase.BucketName(bucket.Name)}
			_, err = server.metabaseDB.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: bucketLocation,
			})
			if err != nil {
				errList.Add(err)
				continue
			}

			empty, err := server.metabaseDB.BucketEmpty(ctx, metabase.BucketEmpty{
				ProjectID:  projectID,
				BucketName: metabase.BucketName(bucket.Name),
			})
			if err != nil {
				errList.Add(err)
				continue
			}
			if !empty {
				errList.Add(metainfo.ErrBucketNotEmpty.New(""))
				continue
			}

			err = server.buckets.DeleteBucket(ctx, []byte(bucket.Name), projectID)
			if err != nil {
				errList.Add(err)
			}
		}
		if errList.Err() != nil {
			return errList.Err()
		}
	}

	err = server.db.Console().APIKeys().DeleteAllByProjectID(ctx, projectID)
	if err != nil {
		return err
	}

	err = server.db.Console().Domains().DeleteAllByProjectID(ctx, projectID)
	if err != nil {
		server.log.Error("failed to delete all domains for project",
			zap.String("project_id", projectID.String()),
			zap.Error(err),
		)
	}

	// We update status to disabled instead of deleting the project
	// to not lose the historical project/user usage data.
	return server.db.Console().Projects().UpdateStatus(ctx, projectID, console.ProjectDisabled)
}

func (server *Server) _updateProjectsUserAgent(ctx context.Context, projectID uuid.UUID, newUserAgent []byte) (err error) {
	err = server.db.Console().Projects().UpdateUserAgent(ctx, projectID, newUserAgent)
	if err != nil {
		return err
	}

	listOptions := buckets.ListOptions{
		Direction: buckets.DirectionForward,
	}

	allowedBuckets := macaroon.AllowedBuckets{
		All: true,
	}

	projectBuckets, err := server.db.Buckets().ListBuckets(ctx, projectID, listOptions, allowedBuckets)
	if err != nil {
		return err
	}

	var errList errs.Group
	for _, bucket := range projectBuckets.Items {
		err = server.db.Buckets().UpdateUserAgent(ctx, projectID, bucket.Name, newUserAgent)
		if err != nil {
			errList.Append(err)
		}

		err = server.db.Attribution().UpdateUserAgent(ctx, projectID, bucket.Name, newUserAgent)
		if err != nil {
			errList.Append(err)
		}
	}

	if errList.Err() != nil {
		return errList.Err()
	}

	return nil
}

func (server *Server) checkInvoicing(ctx context.Context, w http.ResponseWriter, projectID uuid.UUID) (openInvoices bool) {
	year, month, _ := server.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Check if an invoice project record exists already
	err := server.db.StripeCoinPayments().ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
	if errors.Is(err, stripe.ErrProjectRecordExists) {
		record, err := server.db.StripeCoinPayments().
			ProjectRecords().
			Get(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
		if err != nil {
			sendJSONError(w, "unable to get project records", err.Error(), http.StatusInternalServerError)
			return true
		}
		// state = 0 means unapplied and not invoiced yet.
		if record.State == 0 {
			sendJSONError(w, "unapplied project invoice record exist", "", http.StatusConflict)
			return true
		}
		// Record has been applied, so project can be deleted.
		return false
	}
	if err != nil {
		sendJSONError(w, "unable to get project records", err.Error(), http.StatusInternalServerError)
		return true
	}

	return false
}

func (server *Server) checkUsage(ctx context.Context, w http.ResponseWriter, projectID uuid.UUID) (hasUsage bool) {
	year, month, _ := server.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	prj, err := server.db.Console().Projects().Get(ctx, projectID)
	if err != nil {
		sendJSONError(w, "unable to get project details",
			err.Error(), http.StatusInternalServerError)
		return false
	}

	// If user is paid tier, check the usage limit, otherwise it is ok to delete it.
	kind, err := server.db.Console().Users().GetUserKind(ctx, prj.OwnerID)
	if err != nil {
		sendJSONError(w, "unable to project owner tier",
			err.Error(), http.StatusInternalServerError)
		return false
	}
	if kind == console.PaidUser {
		// check current month usage and do not allow deletion if usage exists
		currentUsage, err := server.db.ProjectAccounting().GetProjectTotal(ctx, projectID, firstOfMonth, server.nowFn())
		if err != nil {
			sendJSONError(w, "unable to list project usage", err.Error(), http.StatusInternalServerError)
			return true
		}
		if currentUsage.Storage > 0 || currentUsage.Egress > 0 || currentUsage.SegmentCount > 0 {
			sendJSONError(w, "usage for current month exists", "", http.StatusConflict)
			return true
		}

		// check usage for last month, if exists, ensure we have an invoice item created.
		lastMonthUsage, err := server.db.ProjectAccounting().
			GetProjectTotal(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1))
		if err != nil {
			sendJSONError(w, "error getting project totals",
				"", http.StatusInternalServerError)
			return true
		}
		if lastMonthUsage.Storage > 0 || lastMonthUsage.Egress > 0 || lastMonthUsage.SegmentCount > 0 {
			err = server.db.StripeCoinPayments().
				ProjectRecords().
				Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
			if !errors.Is(err, stripe.ErrProjectRecordExists) {
				sendJSONError(w, "usage for last month exist, but is not billed yet", "", http.StatusConflict)
				return true
			}
		}
	}

	// If we have open invoice items, do not delete the project yet and wait for invoice completion.
	return server.checkInvoicing(ctx, w, projectID)
}

func (server *Server) updatePlacementForProject(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

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

	_, ok = server.placement[placement]
	if !ok {
		sendJSONError(w, "unknown placement parameter", "", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	project, err := server.getProjectByAnyID(ctx, projectUUIDString)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "error getting project",
			"", http.StatusInternalServerError)
		return

	}

	err = server.db.Console().Projects().UpdateDefaultPlacement(ctx, project.ID, placement)
	if err != nil {
		sendJSONError(w, "unable to set geofence for project",
			err.Error(), http.StatusInternalServerError)
	}
}

func bucketNames(buckets []buckets.Bucket) []string {
	var xs []string
	for _, b := range buckets {
		xs = append(xs, b.Name)
	}
	return xs
}

// getProjectByAnyID takes a string version of a project public or private ID. If a valid public or private UUID, the associated project will be returned.
func (server *Server) getProjectByAnyID(ctx context.Context, projectUUIDString string) (p *console.Project, err error) {
	projectID, err := uuidFromString(projectUUIDString)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	p, err = server.db.Console().Projects().GetByPublicID(ctx, projectID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// if failed to get by public ID, try using provided ID as a private ID
		p, err = server.db.Console().Projects().Get(ctx, projectID)
		return p, Error.Wrap(err)
	case err != nil:
		return nil, Error.Wrap(err)
	default:
		return p, nil
	}
}
