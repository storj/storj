// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestBrowserFolderAndDifferentFileSizesUpload(t *testing.T) {
	uitest.Edge(t, func(t *testing.T, ctx *testcontext.Context, planet *uitest.EdgePlanet, browser *rod.Browser) {
		page := openPage(browser, planet.Satellites[0].ConsoleURL())

		// Sign up and login.
		signUpWithUser(t, planet, page)
		loginWithUser(t, planet, page)

		// Navigate into browser with new onboarding.
		page.MustElementR("a", "Skip and go directly to dashboard").MustClick()
		page.MustElementR("p", "Buckets").MustClick()
		wait := page.MustWaitRequestIdle()
		page.MustElementR("p", "demo-bucket").MustClick()
		wait()
		page.MustElementR("label", "I understand, and I have saved the passphrase.").MustClick()
		page.MustElementR("span", "Next >").MustClick()

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

			wait1 := page.MustWaitRequestIdle()
			page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/test0bytes.txt")
			wait1()
			page.MustElementR("span", "testing/testData/test0bytes.txt")
			page.MustElement("#close-modal").MustClick()
			page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
			require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the test0bytes.txt file")
		} else {
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

			wait3 := page.MustWaitRequestIdle()
			page.MustElement("input[aria-roledescription=file-upload]").MustSetFiles("./testdata/test0bytes.txt")
			wait3()
			page.MustElementR("span", "testing/testdata \\(1\\)/test0bytes.txt")
			page.MustElement("#close-modal").MustClick()
			page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
			require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The modal did not open upon clicking the test0bytes.txt file")
		} else {
			wait4 := page.MustWaitRequestIdle()
			page.MustElement("input[aria-roledescription=folder-upload]").MustSetFiles("./testdata")
			wait4()
			page.MustElementR("table > tbody > tr:nth-child(1) > td", "..")
			page.MustElementR("[aria-roledescription=folder]", "testdata \\(1\\)").MustClick()
			page.MustElementR("[aria-roledescription=file]", "test0bytes.txt").MustClick()
			require.Contains(t, page.MustElement("[aria-roledescription=preview-placeholder]").String(), "svg", "The uploaded folder did not upload the files correctly")
		}

		page.MustElement("#close-modal").MustClick()
		page.MustElement("#navigate-back").MustClick()

		// Upload a 0 byte file.
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

			// Upload a 50 MB file.
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
