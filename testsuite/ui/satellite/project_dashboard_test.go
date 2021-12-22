// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestProjectDashboard(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		var (
			satelliteSys = planet.Satellites[0]
			planetUplink = planet.Uplinks[0]
		)

		const (
			bucketName = "testbucket"
			firstPath  = "path"
			secondPath = "another_path"
		)

		user := planetUplink.User[planet.Satellites[0].ID()]

		loginPageURL := planet.Satellites[0].ConsoleURL() + "/login"

		page := openPage(browser, loginPageURL)

		// first time user log in
		page.MustElement("[aria-roledescription=email] input").MustInput(user.Email)
		// we use fullName as a password
		page.MustElement("[aria-roledescription=password] input").MustInput(user.Password)
		page.Keyboard.MustPress(input.Enter)
		waitVueTick(page)

		dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, dashboardTitle, "Dashboard")

		emptyProjectSubtitle := page.MustElement("[aria-roledescription=empty-title]").MustText()
		require.Contains(t, emptyProjectSubtitle, "Welcome to Storj :)\nYou’re ready to experience the future of cloud storage")

		page.MustElementX("(//span[text()=\"Upload\"])").MustClick()
		waitVueTick(page)
		bucketsTitle := page.MustElement("[aria-roledescription=title]").MustText()
		require.Contains(t, bucketsTitle, "Buckets")

		planet.Satellites[0].Orders.Chore.Loop.Pause()
		satelliteSys.Accounting.Tally.Loop.Pause()

		firstSegment := testrand.Bytes(5 * memory.KiB)
		secondSegment := testrand.Bytes(10 * memory.KiB)

		err := planetUplink.Upload(ctx, satelliteSys, bucketName, firstPath, firstSegment)
		require.NoError(t, err)
		err = planetUplink.Upload(ctx, satelliteSys, bucketName, secondPath, secondSegment)
		require.NoError(t, err)

		_, err = planetUplink.Download(ctx, satelliteSys, bucketName, secondPath)
		require.NoError(t, err)

		require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))
		tomorrow := time.Now().Add(24 * time.Hour)
		planet.StorageNodes[0].Storage2.Orders.SendOrders(ctx, tomorrow)

		planet.Satellites[0].Orders.Chore.Loop.TriggerWait()
		satelliteSys.Accounting.Tally.Loop.TriggerWait()

		page.MustElement("[href=\"/new-project-dashboard\"]").MustClick()
		waitVueTick(page)

		withUsageProjectSubtitle := page.MustElement("[aria-roledescription=with-usage-title]").MustText()
		require.Contains(t, withUsageProjectSubtitle, "Your 2 objects are stored in 2 segments around the world")

		graphs := page.MustElements("canvas")
		require.Equal(t, 2, len(graphs))

		page.MustElement("[aria-roledescription=datepicker-toggle]").MustClick()
		page.MustElement("[aria-roledescription=datepicker]")
		page.MustElement("[aria-roledescription=datepicker-toggle]").MustClick()

		page.MustElementX("(//span[text()=\"Upgrade Plan\"])").MustClick()

		// Upgrade to pro account modal
		addPMModalTitle := page.MustElement("[aria-roledescription=modal-title]").MustText()
		require.Contains(t, addPMModalTitle, "Upgrade to Pro Account")
		page.MustElement(".close-cross-container").MustClick()

		infoValues := page.MustElements("[aria-roledescription=info-value]")

		charges := infoValues.First().MustText()
		require.Contains(t, charges, "$0.00")
		objects := infoValues[1].MustText()
		require.Contains(t, objects, "2")
		segments := infoValues.Last().MustText()
		require.Contains(t, segments, "2")

		totalStorageLabel := page.MustElement("[aria-roledescription=total-storage]").MustText()
		require.Contains(t, totalStorageLabel, "Total of 22.27KB")
	})
}
