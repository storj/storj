// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that projects implements console.Projects.
var _ console.Projects = (*projects)(nil)

// implementation of Projects interface repository using spacemonkeygo/dbx orm.
type projects struct {
	db  dbx.Methods
	sdb *satelliteDB
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
	projectsDbx, err := projects.db.All_Project_By_ProjectMember_MemberId_OrderBy_Asc_Project_Name(ctx, dbx.ProjectMember_MemberId(userID[:]))
	if err != nil {
		return nil, err
	}

	return projectsFromDbxSlice(ctx, projectsDbx)
}

// Get is a method for querying project from the database by id.
func (projects *projects) Get(ctx context.Context, id uuid.UUID) (_ *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := projects.db.Get_Project_By_Id(ctx, dbx.Project_Id(id[:]))
	if err != nil {
		return nil, err
	}

	return projectFromDBX(ctx, project)
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

// GetByPublicID is a method for querying project from the database by public_id.
func (projects *projects) GetByPublicID(ctx context.Context, publicID uuid.UUID) (_ *console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := projects.db.Get_Project_By_PublicId(ctx, dbx.Project_PublicId(publicID[:]))
	if err != nil {
		return nil, err
	}

	return projectFromDBX(ctx, project)
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
	createFields.RateLimit = dbx.Project_RateLimit_Raw(project.RateLimit)
	createFields.MaxBuckets = dbx.Project_MaxBuckets_Raw(project.MaxBuckets)
	createFields.PublicId = dbx.Project_PublicId(publicID[:])
	createFields.Salt = dbx.Project_Salt(salt[:])

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

	return projectFromDBX(ctx, createdProject)
}

// Delete is a method for deleting project by Id from the database.
func (projects *projects) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = projects.db.Delete_Project_By_Id(ctx, dbx.Project_Id(id[:]))

	return err
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
		updateFields.UserSpecifiedBandwidthLimit = dbx.Project_UserSpecifiedBandwidthLimit(int64(*project.UserSpecifiedBandwidthLimit))
	}
	if project.SegmentLimit != nil {
		updateFields.SegmentLimit = dbx.Project_SegmentLimit(*project.SegmentLimit)
	}

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(project.ID[:]),
		updateFields)

	return err
}

// UpdateRateLimit is a method for updating projects rate limit.
func (projects *projects) UpdateRateLimit(ctx context.Context, id uuid.UUID, newLimit int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if newLimit < 0 {
		return Error.New("limit can't be set to negative value")
	}

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			RateLimit: dbx.Project_RateLimit(newLimit),
		})

	return err
}

// UpdateBurstLimit is a method for updating projects burst limit.
func (projects *projects) UpdateBurstLimit(ctx context.Context, id uuid.UUID, newLimit int) (err error) {
	defer mon.Task()(&ctx)(&err)

	if newLimit < 0 {
		return Error.New("limit can't be set to negative value")
	}

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			BurstLimit: dbx.Project_BurstLimit(newLimit),
		})

	return err
}

// UpdateBucketLimit is a method for updating projects bucket limit.
func (projects *projects) UpdateBucketLimit(ctx context.Context, id uuid.UUID, newLimit int) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = projects.db.Update_Project_By_Id(ctx,
		dbx.Project_Id(id[:]),
		dbx.Project_Update_Fields{
			MaxBuckets: dbx.Project_MaxBuckets(newLimit),
		})

	return err
}

// List returns paginated projects, created before provided timestamp.
func (projects *projects) List(ctx context.Context, offset int64, limit int, before time.Time) (_ console.ProjectsPage, err error) {
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
func (projects *projects) ListByOwnerID(ctx context.Context, ownerID uuid.UUID, cursor console.ProjectsCursor) (_ console.ProjectsPage, err error) {
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

	countRow := projects.sdb.QueryRowContext(ctx, projects.sdb.Rebind(`
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

	rows, err := projects.sdb.Query(ctx, projects.sdb.Rebind(`
		SELECT id, public_id, name, description, owner_id, rate_limit, max_buckets, created_at,
			(SELECT COUNT(*) FROM project_members WHERE project_id = projects.id) AS member_count
			FROM projects
			WHERE owner_id = ?
			ORDER BY name ASC
			OFFSET ? ROWS
			LIMIT ?
		`), ownerID, page.Offset, page.Limit+1) // add 1 to limit to see if there is another page
	if err != nil {
		return console.ProjectsPage{}, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	count := 0
	projectsToSend := make([]console.Project, 0, page.Limit)
	for rows.Next() {
		count++
		if count == page.Limit+1 {
			// we are done with this page; do not include this project
			page.Next = true
			page.NextOffset = page.Offset + int64(page.Limit)
			break
		}
		var rateLimit, maxBuckets sql.NullInt32
		nextProject := &console.Project{}
		err = rows.Scan(&nextProject.ID, &nextProject.PublicID, &nextProject.Name, &nextProject.Description, &nextProject.OwnerID, &rateLimit, &maxBuckets, &nextProject.CreatedAt, &nextProject.MemberCount)
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

// projectFromDBX is used for creating Project entity from autogenerated dbx.Project struct.
func projectFromDBX(ctx context.Context, project *dbx.Project) (_ *console.Project, err error) {
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

	return &console.Project{
		ID:             id,
		PublicID:       publicID,
		Name:           project.Name,
		Description:    project.Description,
		UserAgent:      userAgent,
		OwnerID:        ownerID,
		RateLimit:      project.RateLimit,
		BurstLimit:     project.BurstLimit,
		MaxBuckets:     project.MaxBuckets,
		CreatedAt:      project.CreatedAt,
		StorageLimit:   (*memory.Size)(project.UsageLimit),
		BandwidthLimit: (*memory.Size)(project.BandwidthLimit),
		SegmentLimit:   project.SegmentLimit,
	}, nil
}

// projectsFromDbxSlice is used for creating []Project entities from autogenerated []*dbx.Project struct.
func projectsFromDbxSlice(ctx context.Context, projectsDbx []*dbx.Project) (_ []console.Project, err error) {
	defer mon.Task()(&ctx)(&err)

	var projects []console.Project
	var errors []error

	// Generating []dbo from []dbx and collecting all errors
	for _, projectDbx := range projectsDbx {
		project, err := projectFromDBX(ctx, projectDbx)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		projects = append(projects, *project)
	}

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
