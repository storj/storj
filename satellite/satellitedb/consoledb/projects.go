// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/private/slices2"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/tagsql"
)

// ensures that projects implements console.Projects.
var _ console.Projects = (*projects)(nil)

var ek = eventkit.Package()

// implementation of Projects interface repository using spacemonkeygo/dbx orm.
type projects struct {
	db   dbx.DriverMethods
	impl dbutil.Implementation
}

// GetAll is a method for querying all projects from the database.
func (projects *projects) GetAll(ctx context.Context) (_ []console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	projectsDbx, err := projects.db.All_Project(ctx)
	if err != nil {
		return nil, err
	}

	return projectsFromDbxSlice(ctx, projectsDbx)
}

// GetOwn is a method for querying all projects created by current user from the database.
func (projects *projects) GetOwn(ctx context.Context, userID uuid.UUID) (_ []console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	projectsDbx, err := projects.db.All_Project_By_OwnerId_OrderBy_Asc_CreatedAt(ctx, dbx.Project_OwnerId(userID[:]))
	if err != nil {
		return nil, err
	}

	return projectsFromDbxSlice(ctx, projectsDbx)
}

// GetCreatedBefore retrieves all projects created before provided date.
func (projects *projects) GetCreatedBefore(ctx context.Context, before time.Time) (_ []console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	projectsDbx, err := projects.db.All_Project_By_CreatedAt_Less_OrderBy_Asc_CreatedAt(ctx, dbx.Project_CreatedAt(before))
	if err != nil {
		return nil, err
	}

	return projectsFromDbxSlice(ctx, projectsDbx)
}

// GetByUserID is a method for querying all projects from the database by userID.
func (projects *projects) GetByUserID(ctx context.Context, userID uuid.UUID) (_ []console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := projects.db.QueryContext(ctx, projects.db.Rebind(`
		SELECT
			projects.id,
			projects.public_id,
			projects.name,
			projects.description,
			projects.owner_id,
			projects.rate_limit,
			projects.max_buckets,
			projects.created_at,
			COALESCE(projects.default_placement, 0),
			COALESCE(projects.default_versioning, 0),
			(SELECT COUNT(*) FROM project_members WHERE project_id = projects.id) AS member_count
		FROM projects
		JOIN project_members ON projects.id = project_members.project_id
		WHERE project_members.member_id = ?
		ORDER BY name ASC
	`), userID)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	nextProject := &console.Project{}
	var rateLimit, maxBuckets sql.NullInt32
	projectsToSend := make([]console.Project, 0)
	for rows.Next() {
		err = rows.Scan(
			&nextProject.ID,
			&nextProject.PublicID,
			&nextProject.Name,
			&nextProject.Description,
			&nextProject.OwnerID,
			&rateLimit,
			&maxBuckets,
			&nextProject.CreatedAt,
			&nextProject.DefaultPlacement,
			&nextProject.DefaultVersioning,
			&nextProject.MemberCount,
		)
		if err != nil {
			return nil, err
		}
		if rateLimit.Valid {
			nextProject.RateLimit = new(int)
			*nextProject.RateLimit = int(rateLimit.Int32)
		}
		if maxBuckets.Valid {
			nextProject.MaxBuckets = new(int)
			*nextProject.MaxBuckets = int(maxBuckets.Int32)
		}
		projectsToSend = append(projectsToSend, *nextProject)
	}

	return projectsToSend, rows.Err()
}

// Get is a method for querying project from the database by id.
func (projects *projects) Get(ctx context.Context, id uuid.UUID) (_ *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := projects.db.Get_Project_By_Id(ctx, dbx.Project_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return ProjectFromDBX(ctx, project)
}

// GetSalt returns the project's salt.
func (projects *projects) GetSalt(ctx context.Context, id uuid.UUID) (salt []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	res, err := projects.db.Get_Project_Salt_By_Id(ctx, dbx.Project_Id(id[:]))
	if err != nil {
		return nil, err
	}

	salt = res.Salt
	if len(salt) == 0 {
		idHash := sha256.Sum256(id[:])
		salt = idHash[:]
	}

	return salt, nil
}

// GetEncryptedPassphrase gets the encrypted passphrase of this project.
// NB: projects that don't have satellite managed encryption will not have this.
func (projects *projects) GetEncryptedPassphrase(ctx context.Context, id uuid.UUID) (encPassphrase []byte, keyID *int, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: add method to DBX after DB freeze is over.

	err = projects.db.QueryRowContext(ctx, `
		SELECT passphrase_enc, passphrase_enc_key_id
		FROM projects
		WHERE id = $1 
	`, id).Scan(&encPassphrase, &keyID)

	return encPassphrase, keyID, err
}

// GetByPublicID is a method for querying project from the database by public_id.
func (projects *projects) GetByPublicID(ctx context.Context, publicID uuid.UUID) (_ *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := projects.db.Get_Project_By_PublicId(ctx, dbx.Project_PublicId(publicID[:]))
	if err != nil {
		return nil, err
	}

	return ProjectFromDBX(ctx, project)
}

// Insert is a method for inserting project into the database.
func (projects *projects) Insert(ctx context.Context, project *console.Project) (_ *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	projectID := project.ID
	if projectID.IsZero() {
		projectID, err = uuid.New()
		if err != nil {
			return nil, err
		}
	}
	publicID, err := uuid.New()
	if err != nil {
		return nil, err
	}

	salt, err := uuid.New()
	if err != nil {
		return nil, err
	}

	createFields := dbx.Project_Create_Fields{}
	if project.UserAgent != nil {
		createFields.UserAgent = dbx.Project_UserAgent(project.UserAgent)
	}
	if project.StorageLimit != nil {
		createFields.UsageLimit = dbx.Project_UsageLimit(project.StorageLimit.Int64())
	}
	if project.BandwidthLimit != nil {
		createFields.BandwidthLimit = dbx.Project_BandwidthLimit(project.BandwidthLimit.Int64())
	}
	if project.SegmentLimit != nil {
		createFields.SegmentLimit = dbx.Project_SegmentLimit(*project.SegmentLimit)
	}
	if project.PassphraseEnc != nil {
		createFields.PassphraseEnc = dbx.Project_PassphraseEnc(project.PassphraseEnc)
	}
	if project.PassphraseEncKeyID != nil {
		createFields.PassphraseEncKeyId = dbx.Project_PassphraseEncKeyId(*project.PassphraseEncKeyID)
	}
	if project.PathEncryption != nil {
		createFields.PathEncryption = dbx.Project_PathEncryption(*project.PathEncryption)
	}
	createFields.RateLimit = dbx.Project_RateLimit_Raw(project.RateLimit)
	createFields.MaxBuckets = dbx.Project_MaxBuckets_Raw(project.MaxBuckets)
	createFields.PublicId = dbx.Project_PublicId(publicID[:])
	createFields.Salt = dbx.Project_Salt(salt[:])
	createFields.DefaultPlacement = dbx.Project_DefaultPlacement(int(project.DefaultPlacement))
	// new projects should have default versioning of Unversioned
	createFields.DefaultVersioning = dbx.Project_DefaultVersioning(int(console.Unversioned))

	createdProject, err := projects.db.Create_Project(ctx,
		dbx.Project_Id(projectID[:]),
		dbx.Project_Name(project.Name),
		dbx.Project_Description(project.Description),
		dbx.Project_OwnerId(project.OwnerID[:]),
		createFields,
	)
	if err != nil {
		return nil, err
	}

	return ProjectFromDBX(ctx, createdProject)
}

// Delete is a method for deleting project by Id from the database.
func (projects *projects) Delete(ctx context.Context, id uuid.UUID) (deleteErr error) {
	defer mon.Task()(&ctx)(&deleteErr)
	// get project info to send to eventkit for historical usage tracking. OK to drop getErr
	project, getErr := projects.Get(ctx, id)

	_, deleteErr = projects.db.Delete_Project_By_Id(ctx, dbx.Project_Id(id[:]))

	if getErr == nil && deleteErr == nil {
		tags := []eventkit.Tag{
			eventkit.String("private-id", project.ID.String()),
			eventkit.String("public-id", project.PublicID.String()),
			eventkit.String("user-agent", string(project.UserAgent)),
			eventkit.String("owner-id", project.OwnerID.String()),
			eventkit.Timestamp("created-at", project.CreatedAt),
			eventkit.Int64("default-placement", int64(project.DefaultPlacement)),
		}
		ek.Event("delete-project", tags...)
		return nil
	}
	if deleteErr != nil {
		return deleteErr
	}
	return nil
}

// Update is a method for updating project entity.
func (projects *projects) Update(ctx context.Context, project *console.Project) (err error) {
	defer mon.Task()(&ctx)(&err)

	updateFields := dbx.Project_Update_Fields{
		Name:        dbx.Project_Name(project.Name),
		Description: dbx.Project_Description(project.Description),
		RateLimit:   dbx.Project_RateLimit_Raw(project.RateLimit),
		BurstLimit:  dbx.Project_BurstLimit_Raw(project.BurstLimit),
	}
	if project.StorageLimit != nil {
		updateFields.UsageLimit = dbx.Project_UsageLimit(project.StorageLimit.Int64())
	}
	if project.UserSpecifiedStorageLimit != nil {
		updateFields.UserSpecifiedUsageLimit = dbx.Project_UserSpecifiedUsageLimit(int64(*project.UserSpecifiedStorageLimit))
	}
	if project.BandwidthLimit != nil {
		updateFields.BandwidthLimit = dbx.Project_BandwidthLimit(project.BandwidthLimit.Int64())
	}
	if project.UserSpecifiedBandwidthLimit != nil {
		updateFields.UserSpecifiedBandwidthLimit = dbx.Project_UserSpecifiedBandwidthLimit(
			int64(*project.UserSpecifiedBandwidthLimit),
		)
	}
	if project.SegmentLimit != nil {
		updateFields.SegmentLimit = dbx.Project_SegmentLimit(*project.SegmentLimit)
	}

	if project.DefaultPlacement > 0 {
		updateFields.DefaultPlacement = dbx.Project_DefaultPlacement(int(project.DefaultPlacement))
	}
	if project.DefaultVersioning > 0 {
		updateFields.DefaultVersioning = dbx.Project_DefaultVersioning(int(project.DefaultVersioning))
	}

	updateFields.PromptedForVersioningBeta = dbx.Project_PromptedForVersioningBeta(project.PromptedForVersioningBeta)

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(project.ID[:]),
		updateFields)

	return err
}

// UpdateRateLimit is a method for updating projects rate limit.
func (projects *projects) UpdateRateLimit(ctx context.Context, id uuid.UUID, newLimit *int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if newLimit != nil && *newLimit < 0 {
		return Error.New("limit can't be set to negative value")
	}

	rateLimit := dbx.Project_RateLimit_Null()
	if newLimit != nil {
		rateLimit = dbx.Project_RateLimit(*newLimit)
	}

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			RateLimit: rateLimit,
		})

	return err
}

// UpdateBurstLimit is a method for updating projects burst limit.
func (projects *projects) UpdateBurstLimit(ctx context.Context, id uuid.UUID, newLimit *int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if newLimit != nil && *newLimit < 0 {
		return Error.New("limit can't be set to negative value")
	}

	burstLimit := dbx.Project_BurstLimit_Null()
	if newLimit != nil {
		burstLimit = dbx.Project_BurstLimit(*newLimit)
	}

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			BurstLimit: burstLimit,
		})

	return err
}

// UpdateBucketLimit is a method for updating projects bucket limit.
func (projects *projects) UpdateBucketLimit(ctx context.Context, id uuid.UUID, newLimit *int) (err error) {
	defer mon.Task()(&ctx)(&err)

	maxBuckets := dbx.Project_MaxBuckets_Null()
	if newLimit != nil {
		if *newLimit < 0 {
			return Error.New("limit can't be set to negative value")
		}

		maxBuckets = dbx.Project_MaxBuckets(*newLimit)
	}

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			MaxBuckets: maxBuckets,
		})

	return err
}

// UpdateAllLimits is a method for updating max buckets, storage, bandwidth, segment, rate, and burst limits.
func (projects *projects) UpdateAllLimits(
	ctx context.Context,
	id uuid.UUID,
	storage, bandwidth, segment *int64,
	buckets, rate, burst *int,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			MaxBuckets:     dbx.Project_MaxBuckets_Raw(buckets),
			UsageLimit:     dbx.Project_UsageLimit_Raw(storage),
			BandwidthLimit: dbx.Project_BandwidthLimit_Raw(bandwidth),
			SegmentLimit:   dbx.Project_SegmentLimit_Raw(segment),
			RateLimit:      dbx.Project_RateLimit_Raw(rate),
			BurstLimit:     dbx.Project_BurstLimit_Raw(burst),
		},
	)

	return err
}

// UpdateLimitsGeneric is a method for updating any or all types of limits on a project.
// ALL limits passed in to the request will be updated i.e. if a limit type is passed in with a null value, that limit will be updated to null.
func (projects *projects) UpdateLimitsGeneric(ctx context.Context, id uuid.UUID, toUpdate []console.Limit) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(toUpdate) == 0 {
		return nil
	}

	updateFields := dbx.Project_Update_Fields{}

	for _, limit := range toUpdate {
		val64 := limit.Value
		var val32 *int
		if val64 != nil {
			newVal := int(*val64)
			val32 = &newVal
		}

		switch limit.Kind {
		case console.StorageLimit:
			updateFields.UsageLimit = dbx.Project_UsageLimit_Raw(val64)
		case console.BandwidthLimit:
			updateFields.BandwidthLimit = dbx.Project_BandwidthLimit_Raw(val64)
		case console.UserSetStorageLimit:
			updateFields.UserSpecifiedUsageLimit = dbx.Project_UserSpecifiedUsageLimit_Raw(val64)
		case console.UserSetBandwidthLimit:
			updateFields.UserSpecifiedBandwidthLimit = dbx.Project_UserSpecifiedBandwidthLimit_Raw(val64)
		case console.SegmentLimit:
			updateFields.SegmentLimit = dbx.Project_SegmentLimit_Raw(val64)
		case console.BucketsLimit:
			updateFields.MaxBuckets = dbx.Project_MaxBuckets_Raw(val32)
		case console.RateLimit:
			updateFields.RateLimit = dbx.Project_RateLimit_Raw(val32)
		case console.BurstLimit:
			updateFields.BurstLimit = dbx.Project_BurstLimit_Raw(val32)
		case console.RateLimitHead:
			updateFields.RateLimitHead = dbx.Project_RateLimitHead_Raw(val32)
		case console.BurstLimitHead:
			updateFields.BurstLimitHead = dbx.Project_BurstLimitHead_Raw(val32)
		case console.RateLimitGet:
			updateFields.RateLimitGet = dbx.Project_RateLimitGet_Raw(val32)
		case console.BurstLimitGet:
			updateFields.BurstLimitGet = dbx.Project_BurstLimitGet_Raw(val32)
		case console.RateLimitPut:
			updateFields.RateLimitPut = dbx.Project_RateLimitPut_Raw(val32)
		case console.BurstLimitPut:
			updateFields.BurstLimitPut = dbx.Project_BurstLimitPut_Raw(val32)
		case console.RateLimitList:
			updateFields.RateLimitList = dbx.Project_RateLimitList_Raw(val32)
		case console.BurstLimitList:
			updateFields.BurstLimitList = dbx.Project_BurstLimitList_Raw(val32)
		case console.RateLimitDelete:
			updateFields.RateLimitDel = dbx.Project_RateLimitDel_Raw(val32)
		case console.BurstLimitDelete:
			updateFields.BurstLimitDel = dbx.Project_BurstLimitDel_Raw(val32)
		default:
			return errs.New("Limit kind not supported in update. No limits updated. Limit kind: %d", limit.Kind)
		}

	}
	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		updateFields,
	)

	return err

}

// UpdateUserAgent is a method for updating projects user agent.
func (projects *projects) UpdateUserAgent(ctx context.Context, id uuid.UUID, userAgent []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			UserAgent: dbx.Project_UserAgent(userAgent),
		})

	return err
}

// UpdateDefaultPlacement is a method to update the project's default placement for new segments.
func (projects *projects) UpdateDefaultPlacement(
	ctx context.Context,
	id uuid.UUID,
	placement storj.PlacementConstraint,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = projects.db.Update_Project_By_Id(
		ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			DefaultPlacement: dbx.Project_DefaultPlacement(int(placement)),
		},
	)

	return err
}

// UpdateDefaultVersioning is a method to update the project's default versioning state for new buckets.
func (projects *projects) UpdateDefaultVersioning(
	ctx context.Context,
	id uuid.UUID,
	defaultVersioning console.DefaultVersioning,
) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = projects.db.Update_Project_By_Id(
		ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			DefaultVersioning: dbx.Project_DefaultVersioning(int(defaultVersioning)),
		},
	)

	return err
}

// List returns paginated projects, created before provided timestamp.
func (projects *projects) List(
	ctx context.Context,
	offset int64,
	limit int,
	before time.Time,
) (_ console.ProjectsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	var page console.ProjectsPage

	dbxProjects, err := projects.db.Limited_Project_By_CreatedAt_Less_OrderBy_Asc_CreatedAt(ctx,
		dbx.Project_CreatedAt(before.UTC()),
		limit+1,
		offset,
	)
	if err != nil {
		return console.ProjectsPage{}, err
	}

	if len(dbxProjects) == limit+1 {
		page.Next = true
		page.NextOffset = offset + int64(limit)

		dbxProjects = dbxProjects[:len(dbxProjects)-1]
	}

	projs, err := projectsFromDbxSlice(ctx, dbxProjects)
	if err != nil {
		return console.ProjectsPage{}, err
	}

	page.Projects = projs
	return page, nil
}

// ListByOwnerID is a method for querying all projects from the database by ownerID. It also includes the number of members for each project.
// cursor.Limit is set to 50 if it exceeds 50.
func (projects *projects) ListByOwnerID(
	ctx context.Context,
	ownerID uuid.UUID,
	cursor console.ProjectsCursor,
) (_ console.ProjectsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	if cursor.Limit > 50 {
		cursor.Limit = 50
	}
	if cursor.Page == 0 {
		return console.ProjectsPage{}, errs.New("page can not be 0")
	}

	page := console.ProjectsPage{
		CurrentPage: cursor.Page,
		Limit:       cursor.Limit,
		Offset:      int64((cursor.Page - 1) * cursor.Limit),
	}

	countRow := projects.db.QueryRowContext(ctx, projects.db.Rebind(`
		SELECT COUNT(*) FROM projects WHERE owner_id = ?
	`), ownerID)
	err = countRow.Scan(&page.TotalCount)
	if err != nil {
		return console.ProjectsPage{}, err
	}
	page.PageCount = int(page.TotalCount / int64(cursor.Limit))
	if page.TotalCount%int64(cursor.Limit) != 0 {
		page.PageCount++
	}

	baseQuery := `
		SELECT
			id,
			public_id,
			name,
			description,
			owner_id,
			rate_limit,
			max_buckets,
			created_at,
			COALESCE(default_placement, 0),
			COALESCE(default_versioning, 0),
			(SELECT COUNT(*) FROM project_members WHERE project_id = projects.id) AS member_count
		FROM projects
		WHERE owner_id = ?
		ORDER BY name ASC
	`
	limit := page.Limit + 1 // add 1 to limit to see if there is another page

	var rows tagsql.Rows
	switch projects.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		rows, err = projects.db.QueryContext(ctx, projects.db.Rebind(
			baseQuery+`
			OFFSET ? ROWS
			LIMIT ?
		`), ownerID, page.Offset, limit) // add 1 to limit to see if there is another page
	case dbutil.Spanner:
		rows, err = projects.db.QueryContext(ctx, projects.db.Rebind(
			baseQuery+`
			LIMIT ?
			OFFSET ?
		`), ownerID, limit, page.Offset) // add 1 to limit to see if there is another page
	default:
		return console.ProjectsPage{}, errs.New("unsupported database dialect: %s", projects.impl)
	}

	if err != nil {
		return console.ProjectsPage{}, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	count := 0
	projectsToSend := make([]console.Project, 0, page.Limit)
	for rows.Next() {
		count++
		if count == limit {
			// we are done with this page; do not include this project
			page.Next = true
			page.NextOffset = page.Offset + int64(page.Limit)
			break
		}
		var rateLimit, maxBuckets sql.NullInt32
		nextProject := &console.Project{}
		err = rows.Scan(
			&nextProject.ID,
			&nextProject.PublicID,
			&nextProject.Name,
			&nextProject.Description,
			&nextProject.OwnerID,
			&rateLimit,
			&maxBuckets,
			&nextProject.CreatedAt,
			&nextProject.DefaultPlacement,
			&nextProject.DefaultVersioning,
			&nextProject.MemberCount,
		)
		if err != nil {
			return console.ProjectsPage{}, err
		}
		if rateLimit.Valid {
			nextProject.RateLimit = new(int)
			*nextProject.RateLimit = int(rateLimit.Int32)
		}
		if maxBuckets.Valid {
			nextProject.MaxBuckets = new(int)
			*nextProject.MaxBuckets = int(maxBuckets.Int32)
		}
		projectsToSend = append(projectsToSend, *nextProject)
	}

	page.Projects = projectsToSend
	return page, rows.Err()
}

// ProjectFromDBX is used for creating Project entity from autogenerated dbx.Project struct.
func ProjectFromDBX(ctx context.Context, project *dbx.Project) (_ *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	if project == nil {
		return nil, errs.New("project parameter is nil")
	}

	id, err := uuid.FromBytes(project.Id)
	if err != nil {
		return nil, err
	}

	var publicID uuid.UUID
	if len(project.PublicId) > 0 {
		publicID, err = uuid.FromBytes(project.PublicId)
		if err != nil {
			return nil, err
		}
	}

	var userAgent []byte
	if len(project.UserAgent) > 0 {
		userAgent = project.UserAgent
	}

	ownerID, err := uuid.FromBytes(project.OwnerId)
	if err != nil {
		return nil, err
	}

	var placement storj.PlacementConstraint
	if project.DefaultPlacement != nil {
		placement = storj.PlacementConstraint(*project.DefaultPlacement)
	}

	return &console.Project{
		ID:                          id,
		PublicID:                    publicID,
		Name:                        project.Name,
		Description:                 project.Description,
		UserAgent:                   userAgent,
		OwnerID:                     ownerID,
		RateLimit:                   project.RateLimit,
		BurstLimit:                  project.BurstLimit,
		RateLimitHead:               project.RateLimitHead,
		BurstLimitHead:              project.BurstLimitHead,
		RateLimitGet:                project.RateLimitGet,
		BurstLimitGet:               project.BurstLimitGet,
		RateLimitPut:                project.RateLimitPut,
		BurstLimitPut:               project.BurstLimitPut,
		RateLimitList:               project.RateLimitList,
		BurstLimitList:              project.BurstLimitList,
		RateLimitDelete:             project.RateLimitDel,
		BurstLimitDelete:            project.BurstLimitDel,
		MaxBuckets:                  project.MaxBuckets,
		CreatedAt:                   project.CreatedAt,
		StorageLimit:                (*memory.Size)(project.UsageLimit),
		UserSpecifiedStorageLimit:   (*memory.Size)(project.UserSpecifiedUsageLimit),
		BandwidthLimit:              (*memory.Size)(project.BandwidthLimit),
		UserSpecifiedBandwidthLimit: (*memory.Size)(project.UserSpecifiedBandwidthLimit),
		SegmentLimit:                project.SegmentLimit,
		DefaultPlacement:            placement,
		DefaultVersioning:           console.DefaultVersioning(project.DefaultVersioning),
		PromptedForVersioningBeta:   project.PromptedForVersioningBeta,
		PathEncryption:              &project.PathEncryption,
		PassphraseEnc:               project.PassphraseEnc,
		PassphraseEncKeyID:          project.PassphraseEncKeyId,
	}, nil
}

// projectsFromDbxSlice is used for creating []Project entities from autogenerated []*dbx.Project struct.
func projectsFromDbxSlice(ctx context.Context, projectsDbx []*dbx.Project) (_ []console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	projects, errors := slices2.ConvertErrs(projectsDbx,
		func(v *dbx.Project) (r console.Project, _ error) {
			p, err := ProjectFromDBX(ctx, v)
			if err != nil {
				return r, err
			}
			return *p, nil
		})
	return projects, errs.Combine(errors...)
}

// GetMaxBuckets is a method to get the maximum number of buckets allowed for the project.
func (projects *projects) GetMaxBuckets(ctx context.Context, id uuid.UUID) (maxBuckets *int, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxRow, err := projects.db.Get_Project_MaxBuckets_By_Id(ctx, dbx.Project_Id(id[:]))
	if err != nil {
		return nil, err
	}
	return dbxRow.MaxBuckets, nil
}

// GetDefaultVersioning is a method to get the default versioning state for new buckets.
func (projects *projects) GetDefaultVersioning(
	ctx context.Context,
	id uuid.UUID,
) (defaultVersioning console.DefaultVersioning, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxRow, err := projects.db.Get_Project_DefaultVersioning_By_Id(ctx, dbx.Project_Id(id[:]))
	if err != nil {
		return 0, err
	}
	return console.DefaultVersioning(dbxRow.DefaultVersioning), nil
}

// UpdateUsageLimits is a method for updating project's bandwidth, storage, and segment limits.
func (projects *projects) UpdateUsageLimits(ctx context.Context, id uuid.UUID, limits console.UsageLimits) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			BandwidthLimit: dbx.Project_BandwidthLimit(limits.Bandwidth),
			UsageLimit:     dbx.Project_UsageLimit(limits.Storage),
			SegmentLimit:   dbx.Project_SegmentLimit(limits.Segment),
		},
	)
	return err
}
