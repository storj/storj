package UITests

import (
	"fmt"
	"gotest.tools/assert"
	"strings"
	"testing"
)

func Test_projectDashboardScreen (t *testing.T) {
	page, browser := login_to_account()
	defer browser.MustClose()

	// checking Dashboard area title
	assert.Assert(t, strings.Contains(page.MustElement(".dashboard-area__title").MustText(),"Dashboard"))

	// storage div
	storageHeader := page.MustElement("p.usage-area__title:nth-of-type(1)").MustText()
	fmt.Println(storageHeader)
	storageRemaining:= page.MustElement("pre.usage-area__remaining:nth-of-type(1)").MustText()
	fmt.Println(storageRemaining)
	storageUsed:= page.MustElement("pre.usage-area__limits-area__title:nth-of-type(1)").MustText()
	fmt.Println(storageUsed)
	storageUsedAmount:= page.MustElementX("(//*[@class=\"usage-area__limits-area__limits\"])[1]").MustText()
	fmt.Println(storageUsedAmount)

	// Bandwidht div
	bandwidthHeader := page.MustElementX("(//*[@class=\"usage-area__title\"])[2]").MustText()
	fmt.Println(bandwidthHeader)
	bandwidthRemaining:= page.MustElementX("(//*[@class=\"usage-area__remaining\"])[2]").MustText()
	fmt.Println(bandwidthRemaining)
	bandwidthUsed:= page.MustElementX("(//*[@class=\"usage-area__limits-area__title\"])[2]").MustText()
	fmt.Println(bandwidthUsed)
	bandwidthUsedAmount:= page.MustElementX("(//*[@class=\"usage-area__limits-area__limits\"])[2]").MustText()
	fmt.Println(bandwidthUsedAmount)

	// Details
	detilsHeader:= page.MustElement("h1.project-summary__title").MustText()
	fmt.Println(detilsHeader)
	userHeader:= page.MustElement("h1.summary-item__title:nth-of-type(1)").MustText()
	fmt.Println(userHeader)
	usersValue:= page.MustElement("p.summary-item__value").MustText()
	fmt.Println(usersValue)
	apiKeysHeader:= page.MustElementX("(//*[@class=\"summary-item__title\"])[2]").MustText()
	fmt.Println(apiKeysHeader)
	apiKeysValue:= page.MustElementX("(//*[@class=\"summary-item__value\"])[2]").MustText()
	fmt.Println(apiKeysValue)
	bucketsHeader:= page.MustElementX("(//*[@class=\"summary-item__title\"])[3]").MustText()
	fmt.Println(bucketsHeader)
	bucketsValue:= page.MustElementX("(//*[@class=\"summary-item__value\"])[3]").MustText()
	fmt.Println(bucketsValue)
	chargesHeader:= page.MustElementX("(//*[@class=\"summary-item__title\"])[4]").MustText()
	fmt.Println(chargesHeader)
	chargesValue:= page.MustElementX("(//*[@class=\"summary-item__value\"])[4]").MustText()
	fmt.Println(chargesValue)

	// project without buckets
	noBucketImage:= page.MustHas("img.no-buckets-area__image")
	fmt.Println(noBucketImage)
	noBucketImageLocation:= page.MustElement("img.no-buckets-area__image").MustAttribute("src")
	fmt.Println(*noBucketImageLocation)
	noBucketsMessage:= page.MustElement("h2.no-buckets-area__message").MustText()
	fmt.Println(noBucketsMessage)
	getStartedButtonLink:= page.MustElement("a.no-buckets-area__first-button").MustAttribute("href")
	fmt.Println(*getStartedButtonLink)
	getStartedButtonText:= page.MustElement("a.no-buckets-area__first-button").MustText()
	fmt.Println(getStartedButtonText)
	docsButtonLink:= page.MustElement("a.no-buckets-area__second-button").MustAttribute("href")
	fmt.Println(*docsButtonLink)
	docsButtonText:= page.MustElement("a.no-buckets-area__second-button").MustText()
	fmt.Println(docsButtonText)
	whycantLink:= page.MustElement("a.no-buckets-area__help").MustAttribute("href")
	fmt.Println(*whycantLink)
	whycantText:= page.MustElement("a.no-buckets-area__help").MustText()
	fmt.Println(whycantText)






	// Output: true
	// Storage
	// 50.00GB Remaining
	// Storage Used
	// 0 / 50.00GB
	// Bandwidth
	// 50.00GB Remaining
	// Bandwidth Used
	// 0 / 50.00GB
	// Details
	// Users
	// 1
	// API Keys
	// 0
	// Buckets
	// 0
	// Estimated Charges
	// $0.00
	// true
	// /static/dist/img/bucket.d8cab0f6.png
	// Create your first bucket to get started.
	// https://documentation.tardigrade.io/api-reference/uplink-cli
	// Get Started
	// https://documentation.tardigrade.io/
	// Visit the Docs
	// https://support.tardigrade.io/hc/en-us/articles/360035332472-Why-can-t-I-upload-from-the-browser-
	// Why can't I upload from the browser?
}

