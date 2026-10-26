// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripe"
)

// usageReportPeriodLayout is the layout of the month of a usage report.
const usageReportPeriodLayout = "2006-01"

// hoursPerMonth is the number of hours in a month that byte-hours and segment-hours are divided by
// to get the usage of a month, the same as invoices.
const hoursPerMonth = 24 * 30

// UsageReportPage holds a page of the usage report of a month.
type UsageReportPage struct {
	// Items holds the usage of each product of the projects in the page, so a project with usage
	// in several products has several items.
	Items []UsageReportItem `json:"items"`
	// NextCursor is the cursor of the next page, or empty if this is the last page.
	NextCursor string `json:"nextCursor"`
}

// UsageReportItem holds the usage and the charges of a project for a product in a month.
//
// The CSV file of a usage report also has items of the adjustments of products with a minimum storage.
// They have a zero ProjectID and an empty ProjectName, since the minimum applies to the storage of all the
// user's projects, and only have the minimum storage and the storage overage usage and charges.
type UsageReportItem struct {
	UserID          uuid.UUID `json:"userID"`
	UserEmail       string    `json:"userEmail"`
	ProjectID       uuid.UUID `json:"projectID"`
	ProjectName     string    `json:"projectName"`
	Period          string    `json:"period"`
	ProductID       int32     `json:"productID"`
	ProductName     string    `json:"productName"`
	DiscountPercent string    `json:"discountPercent"`
	// MinimumStorageBytes is the product's minimum storage, or 0 if it has none.
	MinimumStorageBytes int64 `json:"minimumStorageBytes"`

	StorageGBMonth       float64 `json:"storageGBMonth"`
	EgressGB             float64 `json:"egressGB"`
	SegmentMonth         float64 `json:"segmentMonth"`
	MinObjectSizeGBMonth float64 `json:"minObjectSizeGBMonth"`
	MinRetentionGBMonth  float64 `json:"minRetentionGBMonth"`
	// MinStorageGBMonth is the storage that tops the storage of all the user's projects up to the
	// product's minimum storage.
	MinStorageGBMonth float64 `json:"minStorageGBMonth"`
	// StorageOverageGBMonth is the storage of all the user's projects above the product's minimum storage.
	// It is part of the storage of the projects' items, which charge it at the storage price.
	StorageOverageGBMonth float64 `json:"storageOverageGBMonth"`

	Storage       UsageReportCharge `json:"storage"`
	Egress        UsageReportCharge `json:"egress"`
	Segment       UsageReportCharge `json:"segment"`
	MinObjectSize UsageReportCharge `json:"minObjectSize"`
	MinRetention  UsageReportCharge `json:"minRetention"`
	MinStorage    UsageReportCharge `json:"minStorage"`
	// StorageOverage is how much more than the storage price the storage above the minimum costs.
	StorageOverage UsageReportCharge `json:"storageOverage"`
	Total          UsageReportCharge `json:"total"`
}

// UsageReportCharge holds a charge in cents before and after the product discount.
type UsageReportCharge struct {
	StandardCents int64 `json:"standardCents"`
	DiscountCents int64 `json:"discountCents"`
	BillableCents int64 `json:"billableCents"`
}

// add adds other to the charge.
func (c *UsageReportCharge) add(other UsageReportCharge) {
	c.StandardCents += other.StandardCents
	c.DiscountCents += other.DiscountCents
	c.BillableCents += other.BillableCents
}

// GetUsageReport returns a page of the usage report of a month. The report holds the usage and
// the charges of the active projects of active accounts that have usage in the month, priced by the
// products of the placements they use and discounted by their discounts.
//
// The adjustments of the products with a minimum storage, which apply to all the projects of an account,
// are only in the CSV file of the report. See DownloadUsageReport.
//
// period is the month in "YYYY-MM" format. limit is the number of projects of the page, at most 100.
// cursor is the NextCursor of the previous page, or empty for the first page.
func (s *Service) GetUsageReport(ctx context.Context, period, limit, cursor string) (*UsageReportPage, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiErr := func(status int, err error) (*UsageReportPage, api.HTTPError) {
		return nil, api.HTTPError{Status: status, Err: Error.Wrap(err)}
	}

	since, before, err := s.parseUsageReportPeriod(period)
	if err != nil {
		return apiErr(http.StatusBadRequest, err)
	}

	limitNum, err := strconv.ParseUint(limit, 10, 32)
	if err != nil {
		return apiErr(http.StatusBadRequest, errs.New("invalid limit %q", limit))
	}
	if limitNum == 0 || limitNum > 100 {
		limitNum = 100
	}

	after, err := decodeUsageReportCursor(cursor)
	if err != nil {
		return apiErr(http.StatusBadRequest, err)
	}

	projectsPage, err := s.consoleDB.Projects().ListActiveWithOwners(ctx, s.usageReportProjectsParams(uint(limitNum), after, since, before))
	if err != nil {
		return apiErr(http.StatusInternalServerError, err)
	}

	items := []UsageReportItem{}
	for _, project := range projectsPage.Projects {
		charges, err := s.payments.ProjectProductCharges(ctx, project.ID, project.PublicID, since, before)
		if err != nil {
			return apiErr(http.StatusInternalServerError, err)
		}
		items = append(items, usageReportItems(project, since, charges)...)
	}

	reportPage := &UsageReportPage{Items: items}
	if projectsPage.Next != nil {
		reportPage.NextCursor, err = encodeUsageReportCursor(*projectsPage.Next)
		if err != nil {
			return apiErr(http.StatusInternalServerError, err)
		}
	}

	return reportPage, api.HTTPError{}
}

// DownloadUsageReport writes the usage report of a month as a CSV file, with the totals of the
// report at the end. See GetUsageReport for the content of the report.
//
// Unlike the pages of the report, the rows of an account's projects are followed by a row for each
// product with a minimum storage, with the storage below the minimum or the extra price of the storage
// above it, so that the file adds up to the invoices.
//
// period is the month in "YYYY-MM" format.
func (s *Service) DownloadUsageReport(ctx context.Context, w http.ResponseWriter, period string) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiErr := func(status int, err error) api.HTTPError {
		return api.HTTPError{Status: status, Err: Error.Wrap(err)}
	}

	since, before, err := s.parseUsageReportPeriod(period)
	if err != nil {
		return apiErr(http.StatusBadRequest, err)
	}

	// The first page is listed before writing any header so that its errors are still returned with
	// a proper status code.
	params := s.usageReportProjectsParams(s.usageReportCSVPageLimit, console.ProjectWithOwnerCursor{}, since, before)
	projectsPage, err := s.consoleDB.Projects().ListActiveWithOwners(ctx, params)
	if err != nil {
		return apiErr(http.StatusInternalServerError, err)
	}

	w.Header().Set("Content-Disposition", "attachment; filename=usage-report-"+period+".csv")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// See GetUserUsageReport for why the rows are streamed instead of buffered, and why errors
	// after the first write may not reach the client.
	csvWriter := csv.NewWriter(w)
	if err = csvWriter.Write(usageReportCSVHeader); err != nil {
		return apiErr(http.StatusInternalServerError, err)
	}

	var totals UsageReportItem
	writeItems := func(items []UsageReportItem) error {
		for _, item := range items {
			if err := csvWriter.Write(usageReportCSVRow(item)); err != nil {
				return err
			}
			totals.addUsage(item)
		}
		return nil
	}

	// The projects are sorted by owner, so an owner's storage is added up until the projects of the next
	// owner, and the items of the owner's adjustments follow their last project.
	var lastProject console.ProjectWithOwner
	storageTotals := make(stripe.MinimumStorageTotals)
	for {
		for _, project := range projectsPage.Projects {
			var items []UsageReportItem
			if project.OwnerID != lastProject.OwnerID {
				items, err = s.usageReportUserItems(ctx, lastProject, since, storageTotals)
				if err != nil {
					s.log.Error("error when getting usage report items", zap.Error(err))
					return apiErr(http.StatusInternalServerError, err)
				}
				storageTotals = make(stripe.MinimumStorageTotals)
			}
			lastProject = project

			charges, err := s.payments.ProjectProductCharges(ctx, project.ID, project.PublicID, since, before)
			if err != nil {
				s.log.Error("error when getting usage report items", zap.Error(err))
				return apiErr(http.StatusInternalServerError, err)
			}
			storageTotals.Add(charges)
			items = append(items, usageReportItems(project, since, charges)...)

			if err = writeItems(items); err != nil {
				s.log.Error("error when writing a CSV row", zap.Error(err))
				return apiErr(http.StatusInternalServerError, err)
			}
		}

		if projectsPage.Next == nil {
			break
		}

		params.After = *projectsPage.Next
		projectsPage, err = s.consoleDB.Projects().ListActiveWithOwners(ctx, params)
		if err != nil {
			s.log.Error("error when listing projects for the usage report", zap.Error(err))
			return apiErr(http.StatusInternalServerError, err)
		}
	}

	userItems, err := s.usageReportUserItems(ctx, lastProject, since, storageTotals)
	if err != nil {
		s.log.Error("error when getting usage report items", zap.Error(err))
		return apiErr(http.StatusInternalServerError, err)
	}
	if err = writeItems(userItems); err != nil {
		s.log.Error("error when writing a CSV row", zap.Error(err))
		return apiErr(http.StatusInternalServerError, err)
	}

	for _, row := range usageReportCSVTotals(totals) {
		if err = csvWriter.Write(row); err != nil {
			s.log.Error("error when writing a CSV row", zap.Error(err))
			return apiErr(http.StatusInternalServerError, err)
		}
	}

	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		s.log.Error("error flushing the CSV writer", zap.Error(err))
		return apiErr(http.StatusInternalServerError, err)
	}

	return api.HTTPError{}
}

// TestSetUsageReportCSVPageLimit sets the number of projects listed at once for a usage report CSV file.
// Exported for testing.
func (s *Service) TestSetUsageReportCSVPageLimit(limit uint) {
	s.usageReportCSVPageLimit = limit
}

// parseUsageReportPeriod returns the start and the end of the month of a usage report.
// The current month is allowed, so its report holds the usage so far.
func (s *Service) parseUsageReportPeriod(period string) (since, before time.Time, err error) {
	since, err = time.Parse(usageReportPeriodLayout, period)
	if err != nil {
		return time.Time{}, time.Time{}, errs.New("invalid period %q, expected the YYYY-MM format", period)
	}

	if since.After(s.nowFn().UTC()) {
		return time.Time{}, time.Time{}, errs.New("period %q is in the future", period)
	}

	return since, since.AddDate(0, 1, 0), nil
}

// usageReportProjectsParams returns the parameters to list the projects of a page of a usage report
// that starts after the position after.
func (s *Service) usageReportProjectsParams(limit uint, after console.ProjectWithOwnerCursor, since, before time.Time) console.ListProjectsWithOwnersParams {
	return console.ListProjectsWithOwnersParams{
		Limit:       limit,
		After:       after,
		TenantID:    s.tenantID,
		UsageSince:  since,
		UsageBefore: before,
	}
}

// usageReportCursor is the JSON of a usage report cursor: the position of the last project of a page.
type usageReportCursor struct {
	OwnerEmail string    `json:"e"`
	OwnerID    uuid.UUID `json:"o"`
	Name       string    `json:"n"`
	PublicID   uuid.UUID `json:"p"`
}

// encodeUsageReportCursor returns the cursor of the page after the position.
func encodeUsageReportCursor(position console.ProjectWithOwnerCursor) (string, error) {
	data, err := json.Marshal(usageReportCursor{
		OwnerEmail: position.OwnerNormalizedEmail,
		OwnerID:    position.OwnerID,
		Name:       position.Name,
		PublicID:   position.PublicID,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// decodeUsageReportCursor returns the position of the cursor, or the zero position for an empty cursor.
func decodeUsageReportCursor(cursor string) (console.ProjectWithOwnerCursor, error) {
	if cursor == "" {
		return console.ProjectWithOwnerCursor{}, nil
	}

	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return console.ProjectWithOwnerCursor{}, errs.New("invalid cursor %q", cursor)
	}
	var c usageReportCursor
	// Cursors without the owner ID are from before the projects were ordered by owner, so the projects
	// after them aren't known.
	if err = json.Unmarshal(data, &c); err != nil || c.PublicID.IsZero() || c.OwnerID.IsZero() {
		return console.ProjectWithOwnerCursor{}, errs.New("invalid cursor %q", cursor)
	}

	return console.ProjectWithOwnerCursor{
		OwnerNormalizedEmail: c.OwnerEmail,
		OwnerID:              c.OwnerID,
		Name:                 c.Name,
		PublicID:             c.PublicID,
	}, nil
}

// usageReportUserItems returns the usage report items of the adjustments of the products with a minimum
// storage of the owner of a project, one for each product, from their storage of all their projects.
// Owners who aren't invoiced have none, as on invoices.
func (s *Service) usageReportUserItems(ctx context.Context, project console.ProjectWithOwner, since time.Time, storageTotals stripe.MinimumStorageTotals) ([]UsageReportItem, error) {
	charges := storageTotals.Charges(s.products)
	if len(charges) == 0 {
		return nil, nil
	}

	invoiced, err := s.payments.IsInvoiced(ctx, project.OwnerID)
	if err != nil {
		return nil, err
	}
	if !invoiced {
		return nil, nil
	}

	productIDs := slices.Sorted(maps.Keys(charges))
	items := make([]UsageReportItem, 0, len(productIDs))
	for _, productID := range productIDs {
		charge := charges[productID]
		item := UsageReportItem{
			UserID:          project.OwnerID,
			UserEmail:       project.OwnerEmail,
			Period:          since.Format(usageReportPeriodLayout),
			ProductID:       productID,
			ProductName:     charge.ProductName,
			DiscountPercent: charge.DiscountPercent.String(),

			MinimumStorageBytes: charge.MinimumStorageBytes,

			MinStorageGBMonth:     byteHoursToGBMonth(charge.Storage),
			StorageOverageGBMonth: byteHoursToGBMonth(charge.OverageStorage),

			MinStorage:     discountedCharge(charge.StorageMBMonthCents, charge.DiscountPercent),
			StorageOverage: discountedCharge(charge.OverageMBMonthCents, charge.DiscountPercent),
		}
		item.Total.add(item.MinStorage)
		item.Total.add(item.StorageOverage)

		items = append(items, item)
	}

	return items, nil
}

// usageReportItems returns the usage report items of a project from its charges, one for each product
// it has usage in.
func usageReportItems(project console.ProjectWithOwner, since time.Time, charges map[int32]payments.ProductCharge) []UsageReportItem {
	productIDs := make([]int32, 0, len(charges))
	for productID := range charges {
		productIDs = append(productIDs, productID)
	}
	slices.Sort(productIDs)

	items := make([]UsageReportItem, 0, len(productIDs))
	for _, productID := range productIDs {
		charge := charges[productID]
		item := UsageReportItem{
			UserID:          project.OwnerID,
			UserEmail:       project.OwnerEmail,
			ProjectID:       project.PublicID,
			ProjectName:     project.Name,
			Period:          since.Format(usageReportPeriodLayout),
			ProductID:       productID,
			ProductName:     charge.ProductName,
			DiscountPercent: charge.DiscountPercent.String(),

			MinimumStorageBytes: charge.MinimumStorageBytes,

			StorageGBMonth:       byteHoursToGBMonth(charge.Storage),
			EgressGB:             float64(charge.Egress) / gb,
			SegmentMonth:         charge.SegmentCount / hoursPerMonth,
			MinObjectSizeGBMonth: byteHoursToGBMonth(charge.RemainderStorage),
			MinRetentionGBMonth:  byteHoursToGBMonth(charge.RetentionRemainder),

			Storage:       discountedCharge(charge.StorageMBMonthCents, charge.DiscountPercent),
			Egress:        discountedCharge(charge.EgressMBCents, charge.DiscountPercent),
			Segment:       discountedCharge(charge.SegmentMonthCents, charge.DiscountPercent),
			MinObjectSize: discountedCharge(charge.SmallObjectFeeMBMonthCents, charge.DiscountPercent),
			MinRetention:  discountedCharge(charge.MinimumRetentionFeeMBMonthCents, charge.DiscountPercent),
		}
		for _, c := range []UsageReportCharge{item.Storage, item.Egress, item.Segment, item.MinObjectSize, item.MinRetention} {
			item.Total.add(c)
		}

		items = append(items, item)
	}

	return items
}

// addUsage adds the usage and the charges of other to the item.
func (item *UsageReportItem) addUsage(other UsageReportItem) {
	item.StorageGBMonth += other.StorageGBMonth
	item.EgressGB += other.EgressGB
	item.SegmentMonth += other.SegmentMonth
	item.MinObjectSizeGBMonth += other.MinObjectSizeGBMonth
	item.MinRetentionGBMonth += other.MinRetentionGBMonth
	item.MinStorageGBMonth += other.MinStorageGBMonth
	item.StorageOverageGBMonth += other.StorageOverageGBMonth

	item.Storage.add(other.Storage)
	item.Egress.add(other.Egress)
	item.Segment.add(other.Segment)
	item.MinObjectSize.add(other.MinObjectSize)
	item.MinRetention.add(other.MinRetention)
	item.MinStorage.add(other.MinStorage)
	item.StorageOverage.add(other.StorageOverage)
	item.Total.add(other.Total)
}

// discountedCharge returns the charge of standardCents discounted by discountPercent.
// The discount is rounded to the nearest cent.
func discountedCharge(standardCents int64, discountPercent decimal.Decimal) UsageReportCharge {
	discountCents := payments.DiscountCents(standardCents, discountPercent)
	return UsageReportCharge{
		StandardCents: standardCents,
		DiscountCents: discountCents,
		BillableCents: standardCents - discountCents,
	}
}

// byteHoursToGBMonth converts byte-hours to GB-months.
func byteHoursToGBMonth(byteHours float64) float64 {
	return byteHours / gb / hoursPerMonth
}

// usageReportCSVHeader is the header of the CSV file of a usage report.
var usageReportCSVHeader = []string{
	"email", "project_name", "public_id", "month", "tier",
	"storage_gb_month", "egress_gb", "segment_month", "min_file_size_gb_month", "min_retention_gb_month",
	"min_storage_gb_month", "storage_overage_gb_month",
	"storage standard price", "storage discount", "storage billable price",
	"egress standard price", "egress discount", "egress billable price",
	"segment standard price", "segment discount", "segment billable price",
	"min_file_size standard price", "min_file_size discount", "min_file_size billable price",
	"min_retention standard price", "min_retention discount", "min_retention billable price",
	"min_storage standard price", "min_storage discount", "min_storage billable price",
	"storage_overage standard price", "storage_overage discount", "storage_overage billable price",
	"Grand Total",
}

// usageReportCSVRow returns the CSV row of a usage report item.
func usageReportCSVRow(item UsageReportItem) []string {
	// The items of the adjustments of the products with a minimum storage have no project.
	publicID := ""
	if !item.ProjectID.IsZero() {
		publicID = item.ProjectID.String()
	}
	row := []string{
		csvText(item.UserEmail), csvText(item.ProjectName), publicID, item.Period, csvText(item.ProductName),
	}
	return append(row, usageReportCSVValues(item)...)
}

// csvText returns text so that spreadsheets don't read it as a formula, since emails and project
// names are chosen by users.
func csvText(text string) string {
	if text != "" && strings.ContainsRune("=+-@\t\r", rune(text[0])) {
		return "'" + text
	}
	return text
}

// usageReportCSVTotals returns the CSV rows of the totals of a usage report: a row with the totals of
// each column followed by the totals of the report. All the rows have as many columns as the header.
func usageReportCSVTotals(totals UsageReportItem) [][]string {
	totalRow := func(name string, cents int64) []string {
		row := make([]string, len(usageReportCSVHeader))
		row[0], row[1] = name, formatCents(cents)
		return row
	}
	return [][]string{
		append(make([]string, 5), usageReportCSVValues(totals)...),
		totalRow("Standard billing amount", totals.Total.StandardCents),
		totalRow("Discount", -totals.Total.DiscountCents),
		totalRow("Total amount", totals.Total.BillableCents),
	}
}

// usageReportCSVValues returns the usage and the charges columns of a usage report item.
func usageReportCSVValues(item UsageReportItem) []string {
	values := []string{
		formatUsage(item.StorageGBMonth),
		formatUsage(item.EgressGB),
		formatUsage(item.SegmentMonth),
		formatUsage(item.MinObjectSizeGBMonth),
		formatUsage(item.MinRetentionGBMonth),
		formatUsage(item.MinStorageGBMonth),
		formatUsage(item.StorageOverageGBMonth),
	}
	for _, charge := range []UsageReportCharge{item.Storage, item.Egress, item.Segment, item.MinObjectSize, item.MinRetention, item.MinStorage, item.StorageOverage} {
		values = append(values,
			formatCents(charge.StandardCents),
			formatCents(-charge.DiscountCents),
			formatCents(charge.BillableCents),
		)
	}
	return append(values, formatCents(item.Total.BillableCents))
}

// formatUsage formats a usage amount with up to 6 decimals.
func formatUsage(value float64) string {
	return decimal.NewFromFloat(value).Round(6).String()
}

// formatCents formats an amount of cents as dollars.
func formatCents(cents int64) string {
	return decimal.New(cents, -2).StringFixed(2)
}
