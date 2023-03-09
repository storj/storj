// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestBrowser_Features(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// Navigate into browser with new onboarding.
		page.MustElementR("a", "Skip and go directly to dashboard").MustClick()
		page.MustElementR("p", "Buckets").MustClick()
		page.MustElementR("[aria-roledescription=title]", "Create a bucket")
		page.MustElementR("span", "Continue").MustClick()
		waitVueTick(page)
		page.MustElementR("[aria-roledescription=title]", "Encrypt your bucket")
		page.MustElementR("span", "Continue").MustClick()
		waitVueTick(page)
		page.MustElementR("[aria-roledescription=title]", "Generate a passphrase")
		page.MustElementR("label", "I understand, and I have saved the passphrase.").MustClick()
		page.MustElementR("span", "Continue").MustClick()

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
		folderName := page.MustElement("a[aria-current=\"page\"] a").MustText()
		require.Contains(t, folderName, "folderCreatedThroughInput", "Navigating into the folder `folderCreatedThroughInput` has not been successful")
		page.MustElementR("p", "Drop Files Here to Upload")

		// Attempt to create a new folder but cancel.
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("Hello World!")
		page.MustElementR("button", "Cancel").MustClick()

		// Add a file into folder and check that dropzone is still visible.
		page.MustElementR("button", "Upload").MustClick()
		wait1 := page.MustWaitRequestIdle()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/img.png")
		wait1()
		page.MustElementR("span", "folderCreatedThroughInput/img.png")
		page.MustElement("#close-modal").MustClick()
		page.MustElementR("[aria-roledescription=file]", "img.png")
		page.MustElementR("p", "Drop Files Here to Upload")

		// Click on the file name.
		page.MustElementR("[aria-roledescription=file]", "img.png").MustClick()
		page.MustElement("[aria-roledescription=image-preview]")

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
		page.MustElement("[aria-roledescription=image-preview]")
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
		page.MustElementR("button", "Upload").MustClick()
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
		page.MustElementR("a[href=\"/buckets/upload/folderCreatedThroughInput/\"]", "folderCreatedThroughInput")

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
		page.MustElement("[href=\"/buckets/upload/Свобода/\"]")
		page.MustElementR("p", "Drop Files Here to Upload")
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("Свобода")
		page.MustElementR("button", "Save Folder").MustClick()
		page.MustElementR("[aria-roledescription=folder]", "Свобода")

		// upload a video.
		page.MustElementR("button", "Upload").MustClick()
		wait3 := page.MustWaitRequestIdle()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/movie.mp4")
		wait3()
		page.MustElementR("span", "Свобода/movie.mp4")
		page.MustElement("#close-modal").MustClick()
		page.MustElementR("[aria-roledescription=file]", "movie.mp4")
		page.MustElementR("[aria-roledescription=file-size]", "1.48 kB")
		page.MustElement("[aria-roledescription=file-upload-date]")
		page.MustElementR("[aria-roledescription=file]", "movie.mp4").MustClick()
		page.MustElement("[aria-roledescription=video-preview]")
		page.MustElement("#close-modal").MustClick()

		// Upload an audio file.
		wait4 := page.MustWaitRequestIdle()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/audio.mp3")
		wait4()
		page.MustElementR("[aria-roledescription=file]", "audio.mp3").MustClick()
		page.MustElement("[aria-roledescription=audio-preview]")
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

func TestBrowser_FolderAndDifferentFileSizesUpload(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// Navigate into browser with new onboarding.
		page.MustElementR("a", "Skip and go directly to dashboard").MustClick()
		page.MustElementR("p", "Buckets").MustClick()
		page.MustElementR("[aria-roledescription=title]", "Create a bucket")
		page.MustElementR("span", "Continue").MustClick()
		waitVueTick(page)
		page.MustElementR("[aria-roledescription=title]", "Encrypt your bucket")
		page.MustElementR("span", "Continue").MustClick()
		waitVueTick(page)
		page.MustElementR("[aria-roledescription=title]", "Generate a passphrase")
		page.MustElementR("label", "I understand, and I have saved the passphrase.").MustClick()
		page.MustElementR("span", "Continue").MustClick()

		// Verify that browser component has loaded and that the dropzone is present.
		page.MustElementR("p", "Drop Files Here to Upload")

		// Create a Folder.
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("testing")
		page.MustElementR("button", "Save Folder").MustClick()
		page.MustElementR("[aria-roledescription=folder]", "testing").MustClick()

		// Attempt to create a folder with spaces.
		page.MustElementR("button", "New Folder").MustClick()
		page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("   ")
		require.Equal(t, "true", page.MustElementR("button", "Save Folder").MustProperty("disabled").Str(), "Folder is not disabled on invalid folder name with spaces")
		require.Equal(t, "   ", page.MustElement("[placeholder=\"Name of the folder\"]").MustText(), "Folder input does not contain the empty invalid name")
		page.MustElementR("button", "Cancel").MustClick()

		// Upload a folder (folder upload doesn't work when headless).
		if os.Getenv("STORJ_TEST_SHOW_BROWSER") == "" {
			// Create folder
			page.MustElementR("button", "New Folder").MustClick()
			page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("testData")
			page.MustElementR("button", "Save Folder").MustClick()

			// Navigate into folder and upload file.
			page.MustElementR("[aria-roledescription=folder]", "testData").MustClick()
			page.MustElement("[href=\"/objects/upload/testing/testData/\"]")
			page.MustElementR("p", "Drop Files Here to Upload").MustText()

			// Attempt to create a folder with spaces.
			page.MustElementR("button", "New Folder").MustClick()
			page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("   ")
			require.Equal(t, "true", page.MustElementR("button", "Save Folder").MustProperty("disabled").Str(), "Folder is not disabled on invalid folder name with spaces")
			require.Equal(t, "   ", page.MustElement("[placeholder=\"Name of the folder\"]").MustText(), "Folder input does not contain the empty invalid name")
			page.MustElementR("button", "Cancel").MustClick()

			page.MustElementR("button", "Upload").MustClick()
			wait1 := page.MustWaitRequestIdle()
			page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/test0bytes.txt")
			wait1()
			page.MustElementR("span", "testing/testData/test0bytes.txt")
			page.MustElement("#close-modal").MustClick()
			page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
			require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the test0bytes.txt file")
		} else {
			page.MustElementR("button", "Upload").MustClick()
			wait2 := page.MustWaitRequestIdle()
			page.MustElement("input[aria-roledescription=folder-upload]").MustSetFiles("./testdata")
			wait2()
			page.MustElementR("[aria-roledescription=folder]", "testdata").MustClick()
			page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
			require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The uploaded folder did not upload the files correctly")
		}

		page.MustElement("#close-modal").MustClick()
		page.MustElement("#navigate-back").MustClick()

		// Upload duplicate folder (folder upload doesn't work when headless).
		if os.Getenv("STORJ_TEST_SHOW_BROWSER") == "" {
			// Create folder.
			page.MustElementR("button", "New Folder").MustClick()
			page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("testdata (1)")
			page.MustElementR("button", "Save Folder").MustClick()

			// Navigate into folder and upload file.
			page.MustElementR("[aria-roledescription=folder]", "testdata \\(1\\)").MustClick()
			page.MustElement("[href=\"/objects/upload/testing/testdata (1)/\"]")
			page.MustElementR("p", "Drop Files Here to Upload")

			// Attempt to create a folder with spaces.
			page.MustElementR("button", "New Folder").MustClick()
			page.MustElement("[placeholder=\"Name of the folder\"]").MustInput("   ")
			require.Equal(t, "true", page.MustElementR("button", "Save Folder").MustProperty("disabled").Str(), "Folder is not disabled on invalid folder name with spaces")
			require.Equal(t, "   ", page.MustElement("[placeholder=\"Name of the folder\"]").MustText(), "Folder input does not contain the empty invalid name")
			page.MustElementR("button", "Cancel").MustClick()

			page.MustElementR("button", "Upload").MustClick()
			wait3 := page.MustWaitRequestIdle()
			page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/test0bytes.txt")
			wait3()
			page.MustElementR("span", "testing/testdata \\(1\\)/test0bytes.txt")
			page.MustElement("#close-modal").MustClick()
			page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
			require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the test0bytes.txt file")
		} else {
			page.MustElementR("button", "Upload").MustClick()
			wait4 := page.MustWaitRequestIdle()
			page.MustElement("input[aria-roledescription=folder-upload]").MustSetFiles("./testdata")
			wait4()
			page.MustElementR("table > tbody > tr:nth-child(1) > td", "..")
			page.MustElementR("[aria-roledescription=folder]", "testdata \\(1\\)").MustClick()
			waitVueTick(page)
			page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
			require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The uploaded folder did not upload the files correctly")
		}

		page.MustElement("#close-modal").MustClick()
		page.MustElement("#navigate-back").MustClick()

		// Upload a 0 byte file.
		page.MustElementR("button", "Upload").MustClick()
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/test0bytes.txt")
		page.MustElementR("span", "testing/test0bytes.txt")
		page.MustElement("#close-modal").MustClick()
		page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
		require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the 0 byte file")
		page.MustElement("#close-modal").MustClick()

		// Upload duplicate 0 byte file.
		page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/test0bytes.txt")
		page.MustElementR("[aria-roledescription=file]", "test0bytes \\(1\\).txt").MustClick()
		require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the duplicate file")
		page.MustElement("#close-modal").MustClick()

		if !testing.Short() {
			slowpage := page.Sleeper(uitest.MaxDuration(20 * time.Second))

			// Upload a 5 MB file.
			testFile := generateEmptyFile(t, ctx, "testFile", 5*memory.MiB)
			wait5 := slowpage.MustWaitRequestIdle()
			slowpage.MustElement("input[aria-roledescription=file-upload]").MustSetFiles(testFile)
			wait5()
			slowpage.MustElementR("[aria-roledescription=file]", "testFile").MustClick()
			require.Contains(t, slowpage.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the 50 MB file")
			slowpage.MustElement("#close-modal").MustClick()

			// Attempt to upload a large file and cancel the upload after a few segments have been uploaded successfully.
			testFile2 := generateEmptyFile(t, ctx, "testFile2", 130*memory.MiB)
			slowpage.MustElement("input[aria-roledescription=file-upload]").MustSetFiles(testFile2)
			require.Equal(t, " testing/testFile2", slowpage.MustElement("[aria-roledescription=file-uploading]").MustText(), "The testFile2 file has not started uploading")
			slowpage.MustElementR("[aria-roledescription=files-uploading-count]", "1 file waiting to be uploaded...")
			slowpage.MustElementR("[aria-roledescription=progress-bar]", "1")
			slowpage.MustElementR("button", "Cancel").MustClick()
			slowpage.MustElementR("table > tbody > tr:nth-child(6) > td > span", "testFile")
			slowpage.MustElementR("[aria-roledescription=file]", "testFile").MustClick()
			slowpage.MustElement("#close-modal").MustClick()

			// Upload a 130MB file.
			page.MustElementR("button", "Upload").MustClick()
			wait6 := slowpage.MustWaitRequestIdle()
			slowpage.MustElement("input[aria-roledescription=file-upload]").MustSetFiles(testFile2)
			require.Equal(t, " testing/testFile2", slowpage.MustElement("[aria-roledescription=file-uploading]").MustText(), "The testFile2 file has not started uploading")
			slowpage.MustElementR("[aria-roledescription=files-uploading-count]", "1 file waiting to be uploaded...")
			wait6()
			slowpage.MustElementR("[aria-roledescription=file]", "testFile2").MustClick()
			require.Contains(t, slowpage.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the 130MB file")
			slowpage.MustElement("#close-modal").MustClick()
		}

		// Navigate out of nested folder and delete everything.
		page.MustElement("#navigate-back").MustClick()
		page.MustElement("button[aria-roledescription=dropdown]").MustClick()
		page.MustElementR("button", "Delete").MustClick()
		wait7 := page.MustWaitRequestIdle()
		page.MustElementR("button", "Yes").MustClick()
		wait7()
	})
}

func TestBrowser_OpenBucket(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// Navigate into browser with new onboarding.
		page.MustElementR("a", "Skip and go directly to dashboard").MustClick()
		page.MustElementR("p", "Buckets").MustClick()
		page.MustElementR("[aria-roledescription=title]", "Create a bucket")
		page.MustElementR("span", "Continue").MustClick()
		waitVueTick(page)
		page.MustElementR("[aria-roledescription=title]", "Encrypt your bucket")
		page.MustElementR("span", "Continue").MustClick()
		waitVueTick(page)
		page.MustElementR("[aria-roledescription=title]", "Generate a passphrase")
		page.MustElementR("label", "I understand, and I have saved the passphrase.").MustClick()
		page.MustElementR("span", "Continue").MustClick()

		// Verify that browser component has loaded and that the dropzone is present.
		page.MustElementR("p", "Drop Files Here to Upload")

		// Navigate to buckets view
		page.MustElementR("p", "<- Back to Buckets").MustClick()
		waitVueTick(page)

		// Open bucket
		page.MustElementR("p", "demo-bucket").MustClick()
		page.MustElementR("h1", "Open a Bucket")
		input := page.MustElement("[aria-roledescription=bucket] input").MustText()
		require.Equal(t, "demo-bucket", input)

		page.MustElement("[aria-roledescription=passphrase] input").MustInput("passphrase")
		wait := page.MustWaitRequestIdle()
		page.MustElementR("span", "Continue ->").MustClick()
		wait()

		// Verify that browser component has loaded and that the dropzone is present.
		page.MustElementR("p", "Drop Files Here to Upload")
	})
}
