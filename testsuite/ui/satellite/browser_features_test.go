// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"fmt"
	"testing"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestBrowserFeatures(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// Navigate into browser with new onboarding.
		page.MustElementR("a", "Skip and go directly to dashboard").MustClick()
		page.MustElementR("p", "Objects").MustClick()
		page.MustElementR("label", "I understand, and I have saved the passphrase.").MustClick()
		page.MustElementR("span", "Next >").MustClick()

		// Verify that browser component has loaded and that the dropzone is present.
		page.MustElementR("p", "Drop Files Here to Upload")

		// Attempt to create an invalid folder.
		page.MustElementR("button", "New Folder").MustClick()
		folderInput := page.MustElement("[placeholder=\"Name of the folder\"]")
		folderInput.MustInput("...")
		page.MustElementR("button", "Save Folder").MustProperty("disabled")
		require.Equal(t, "...", folderInput.MustText(), "Folder input does not contain the `...` invalid name")

		// Create a folder.
		err := folderInput.SelectAllText()
		require.NoError(t, err)

		folderInput.MustInput("folderCreatedThroughInput")
		page.MustElementR("button", "Save Folder").MustClick()
		page.MustElementR("[aria-roledescription=folder]", "folderCreatedThroughInput")

		// Navigate into the folder and make sure the dropzone is visible.
		page.MustElementR("[aria-roledescription=folder]", "folderCreatedThroughInput").MustClick()
		require.Equal(t, " folderCreatedThroughInput", page.MustElement("a[aria-current=\"page\"]").MustText(), "Navigating into the folder `folderCreatedThroughInput` has not been successful")
		page.MustElementR("p", "Drop Files Here to Upload")

		// Attempt to create a new folder but cancel.
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("Hello World!")
		page.MustElementR("button", "Cancel").MustClick()

		// Add a file into folder and check that dropzone is still visible.
		wait := page.MustWaitRequestIdle()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/img.png")
		wait()
		page.MustElementR("span", "folderCreatedThroughInput/img.png")
		page.MustElement("#close-modal").MustClick()
		page.MustElementR("[aria-roledescription=file]", "img.png")
		page.MustElementR("p", "Drop Files Here to Upload")

		// Click on the file name.
		page.MustElementR("[aria-roledescription=file]", "img.png").MustClick()
		require.Contains(t, page.MustElement("[aria-roledescription=image-preview]").MustProperty("src").Str(), "img.png", "The modal did not open on file click")

		// Share a file.
		page.MustElementR("span", "Share").MustClick()
		page.MustElement("#generateShareLink")
		page.MustElement("#close-modal").MustClick()

		// Click on the hamburger and share.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Share").MustClick()
		page.MustElement("#btn-copy-link")
		page.MustElement("[aria-roledescription=close-share-modal]").MustClick()

		// Click on the hamburger and then details.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Details").MustClick()
		require.Contains(t, page.MustElement("[aria-roledescription=image-preview]").MustProperty("src").Str(), "img.png", "The dropdown details functionality is not working")
		page.MustElementR("span", "Share").MustClick()
		page.MustElement("#generateShareLink")
		page.MustElement("#close-modal").MustClick()

		// Use the `..` to navigate out of the folder.
		page.MustElement("#navigate-back").MustClick()
		page.MustElementR("a[aria-current=page]", "demo-bucket")

		// Add another folder.
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("go-rod-test3")
		page.MustElementR("button", "Save Folder").MustClick()
		page.MustElementR("[aria-roledescription=folder]", "go-rod-test3")

		// Add two files.
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/img2.png")
		page.MustElement("#close-modal").MustClick()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/img.png")
		page.MustElementR("[aria-roledescription=file]", "img2.png")
		page.MustElementR("[aria-roledescription=file]", "img.png")

		// Sort folders/files (by name, size, and date).
		require.Equal(t, " folderCreatedThroughInput", page.MustElement("table > tbody > tr:nth-child(1) > td").MustText(), "The automatic sorting by name for folders is not working")
		require.Equal(t, " img.png", page.MustElement("table > tbody > tr:nth-child(3) > td").MustText(), "The automatic sorting by name for files is not working")
		page.MustElementR("th", "Name").MustClick()
		require.Equal(t, " go-rod-test3", page.MustElement("table > tbody > tr:nth-child(1) > td").MustText(), "Sorting by name is not working for folders")
		require.Equal(t, " img2.png", page.MustElement("table > tbody > tr:nth-child(3) > td").MustText(), "Sorting by name is not working for files")
		// sort by size and date still left to do.

		// Single folder select.
		page.MustElement("table > tbody > tr:nth-child(1)").MustClick()
		require.Contains(t, page.MustElement("table > tbody > tr:nth-child(1)").String(), ".selected-row", "The clicked folder row has not been selected properly")

		// Multifolder unselect.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		require.Equal(t, "false", fmt.Sprint(page.MustElement("table > tbody > tr:nth-child(1)").MustHas(".selected-row")), "Multiple selected folders were not unselected successfully")

		// Single file select.
		page.MustElement("table > tbody > tr:nth-child(3)").MustClick()
		require.Contains(t, page.MustElement("table > tbody > tr:nth-child(3)").String(), ".selected-row", "Single file select is not working properly")
		// Multifile select **CAN'T SIMULATE MULTIPLE FILE SELECT YET**.

		// Multifile unselect.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		require.Equal(t, "false", fmt.Sprint(page.MustElement("table > tbody > tr:nth-child(3)").MustHas(".selected-row")), "Multiple selected files were not unselected successfully")

		// Select file and folders **CAN'T SIMULATE MULTIPLE FILE/FOLDERS SELECT YET**.

		// Navigate into folders and use the breadcrumbs to navigate out.
		page.MustElementR("[aria-roledescription=folder]", "go-rod-test3").MustClick()
		page.MustElement("#navigate-back").MustClick()
		page.MustElementR("a[aria-current=page]", "demo-bucket")

		// Cancel folder deletion by way of hamburger.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Delete").MustClick()
		page.MustElementR("button", "No").MustClick()
		page.MustElementR("a", "go-rod-test3")

		// Delete a folder by clicking on hamburger.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Delete").MustClick()
		page.MustElementR("button", "Yes").MustClick()
		page.MustElementR("table > tbody > tr:nth-child(1) > td", "folderCreatedThroughInput")

		// Cancel folder deletion by way of trashcan.
		page.MustElement("tr[scope=\"row\"]").MustClick()
		page.MustElement("#header-delete").MustClick()
		page.MustElementR("button", "No").MustClick()
		page.MustElementR("a[href=\"/objects/upload/folderCreatedThroughInput/\"]", "folderCreatedThroughInput")

		// Delete a folder by selecting and clicking on trashcan.
		page.MustElement("tr[scope=row]").MustClick()
		page.MustElement("#header-delete").MustClick()
		page.MustElementR("button", "Yes").MustClick()
		page.MustElementR("table > tbody > tr:nth-child(1) > td", "img2.png")

		// Cancel file deletion by way of hamburger.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Delete").MustClick()
		page.MustElementR("button", "No").MustClick()
		require.Equal(t, " img2.png", page.MustElement("table > tbody > tr:nth-child(1) > td").MustText(), "File deletion cancellation by way of hamburger is not working")

		// Delete a file by clicking on the hamburger.
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Delete").MustClick()
		page.MustElementR("button", "Yes").MustClick()
		page.MustElementR("table > tbody > tr:nth-child(1) > td", "img.png")

		// Cancel file deletion by way of trashcan.
		page.MustElement("tr[scope=row]").MustClick()
		page.MustElement("#header-delete").MustClick()
		page.MustElementR("button", "No").MustClick()
		require.Equal(t, " img.png", page.MustElement("table > tbody > tr:nth-child(1) > td").MustText(), "File cancellation by way of trashcan is not working")

		// Delete a file by clicking on the row and clicking on the trashcan.
		wait2 := page.MustWaitRequestIdle()
		page.MustElement("tr[scope=row]").MustClick()
		page.MustElement("#header-delete").MustClick()
		page.MustElementR("button", "Yes").MustClick()
		page.MustElementR("p", "Drop Files Here to Upload")
		wait2()

		// Delete multiple folders by selection **SELECTION NOT WORKING**.

		// Delete multiple files by selection **SELECTION NOT WORKING**.

		// Empty out entire folder.

		// Attempt to create a folder with spaces.
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("   ")
		page.MustElementR("button", "Save Folder").MustProperty("disabled")
		require.Equal(t, "   ", page.MustElement("[placeholder=\"Name of the folder\"]").MustText(), "Folder input does not contain the empty invalid name")
		page.MustElementR("button", "Cancel").MustClick()

		// Create Folder with special characters.
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("Свобода")
		page.MustElementR("button", "Save Folder").MustClick()
		page.MustElementR("[aria-roledescription=folder]", "Свобода")

		// Navigate into folder and create another folder of the same name, and check that the dropzone is present.
		page.MustElementR("[aria-roledescription=folder]", "Свобода").MustClick()
		page.MustElement("[href=\"/objects/upload/Свобода/\"]")
		page.MustElementR("p", "Drop Files Here to Upload")
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("Свобода")
		page.MustElementR("button", "Save Folder").MustClick()
		page.MustElementR("[aria-roledescription=folder]", "Свобода")

		// upload a video.
		wait3 := page.MustWaitRequestIdle()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/movie.mp4")
		wait3()
		page.MustElementR("span", "Свобода/movie.mp4")
		page.MustElement("#close-modal").MustClick()
		page.MustElementR("[aria-roledescription=file]", "movie.mp4")
		page.MustElementR("[aria-roledescription=file-size]", "1.48 kB")
		page.MustElement("[aria-roledescription=file-upload-date]")
		page.MustElementR("[aria-roledescription=file]", "movie.mp4").MustClick()
		require.Contains(t, page.MustElement("[aria-roledescription=video-preview]").MustProperty("src").Str(), "movie.mp4", "The modal did not open on video file click")
		page.MustElement("#close-modal").MustClick()

		// Upload an audio file.
		wait4 := page.MustWaitRequestIdle()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/audio.mp3")
		wait4()
		page.MustElementR("[aria-roledescription=file]", "audio.mp3").MustClick()
		require.Contains(t, page.MustElement("[aria-roledescription=audio-preview]").MustProperty("src").Str(), "audio.mp3", "The modal did not open on video file click")
		page.MustElement("#close-modal").MustClick()

		// Navigate out of nested folder and delete everything.
		page.MustElement("#navigate-back").MustClick()
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Delete").MustClick()
		wait5 := page.MustWaitRequestIdle()
		page.MustElementR("button", "Yes").MustClick()
		wait5()
	})
}
