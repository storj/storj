// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
)

func navigateToBilling(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser, signupQuery string) *rod.Page {
	signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup" + signupQuery
	fullName := "John Doe"
	emailAddress := "test@email.test"
	password := "qazwsx123"

	page := openPage(browser, signupPageURL)

	page.MustElement("[aria-roledescription=name] input").MustInput(fullName)
	page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
	page.MustElement("[aria-roledescription=password] input").MustInput(password)
	page.MustElement("[aria-roledescription=retype-password] input").MustInput(password)
	page.MustElement(".checkmark").MustClick()
	page.Keyboard.MustPress(input.Enter)
	waitVueTick(page)
	confirmAccountEmailMessage := page.MustElement("[aria-roledescription=title]").MustText()
	require.Contains(t, confirmAccountEmailMessage, "You're almost there!")

	// first time user logs in
	page.MustElement("[href=\"/login\"]").MustClick()
	waitVueTick(page)
	page.MustElement("[aria-roledescription=email] input").MustInput(emailAddress)
	page.MustElement("[aria-roledescription=password] input").MustInput(password)
	page.Keyboard.MustPress(input.Enter)
	waitVueTick(page)

	// skip onboarding process
	page.MustElement("[href=\"/new-project-dashboard\"]").MustClick()
	dashboardTitle := page.MustElement("[aria-roledescription=title]").MustText()
	require.Contains(t, dashboardTitle, "Dashboard")

	// go to billing page
	page.MustElement("[aria-roledescription=account-area]").MustClick()
	page.MustElementR("p", "Billing").MustClick()
	waitVueTick(page)

	return page
}

func TestCouponCodes(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		page := navigateToBilling(t, ctx, planet, browser, "")

		couponText := page.MustElement(".coupon-area__container__text-container").MustText()
		require.Contains(t, couponText, "Add a Coupon to Get Started")
		require.Contains(t, couponText, "Your coupon will show up here.")

		// Adding a valid promo code (promo1 and promo2 are defined in satellite/payments/stripecoinpayments/stripemock.go)
		{
			page.MustElementR("span", "Add Coupon Code").MustClick()
			page.MustElement("input[placeholder=\"Enter Coupon Code\"]").MustInput("promo1")
			page.MustElementR("span", "Apply Coupon Code").MustClick()
			page.MustElementR("p", "Successfully applied coupon code")

			page.MustElement(".add-coupon__close-icon").MustClick()
			couponText := page.MustElement(".coupon-area__container__text-container").MustText()
			require.Contains(t, couponText, "Test Promo Code 1")
			require.Contains(t, couponText, "$5 off")
		}

		// Attempting replace with invalid promo code
		{
			page.MustElementR("span", "Add Coupon Code").MustClick()
			page.MustElement("input[placeholder=\"Enter Coupon Code\"]").MustInput("notpromo1")
			page.MustElementR("span", "Apply Coupon Code").MustClick()

			page.MustElementR(".add-coupon__confirm-message", "Are you sure.*remove.*current coupon.*replace.*new coupon")
			page.MustElementR("span", "Back").MustClick()

			page.MustElement("input[placeholder=\"Enter Coupon Code\"]").MustInput("notpromo1")
			page.MustElementR("span", "Apply Coupon Code").MustClick()
			page.MustElementR(".add-coupon__confirm-message", "Are you sure.*remove.*current coupon.*replace.*new coupon")
			page.MustElementR("span", "Yes").MustClick()

			page.MustElementR("p", "Could not apply coupon code")

			// old coupon should still be applied
			page.MustElement(".add-coupon__close-icon").MustClick()
			couponText := page.MustElement(".coupon-area__container__text-container").MustText()
			require.Contains(t, couponText, "Test Promo Code 1")
			require.Contains(t, couponText, "$5 off")
		}

		// Replacing with a valid promo code
		{
			page.MustElementR("span", "Add Coupon Code").MustClick()
			page.MustElement("input[placeholder=\"Enter Coupon Code\"]").MustInput("promo2")
			page.MustElementR("span", "Apply Coupon Code").MustClick()
			page.MustElementR(".add-coupon__confirm-message", "Are you sure.*remove.*current coupon.*replace.*new coupon")
			page.MustElementR("span", "Yes").MustClick()
			page.MustElementR("p", "Successfully applied coupon code")

			page.MustElement(".add-coupon__close-icon").MustClick()
			couponText := page.MustElement(".coupon-area__container__text-container").MustText()
			require.Contains(t, couponText, "Test Promo Code 2")
			require.Contains(t, couponText, "50% off")

		}

		// Rate limit
		{
			page.MustElementR("span", "Add Coupon Code").MustClick()
			for i := 0; i < 3; i++ {
				page.MustElement("input[placeholder=\"Enter Coupon Code\"]").MustInput("promo1")
				page.MustElementR("span", "Apply Coupon Code").MustClick()
				page.MustElementR("span", "Yes").MustClick()
			}
			page.MustElementR("p", "You've exceeded limit of attempts")
		}
	})
}

func TestCouponCode_SignupGood(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		page := navigateToBilling(t, ctx, planet, browser, "/?promo=promo1")

		couponText := page.MustElement(".coupon-area__container__text-container").MustText()
		require.Contains(t, couponText, "Test Promo Code 1")
		require.Contains(t, couponText, "$5 off")

	})
}

func TestCouponCode_SignupBad(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		page := navigateToBilling(t, ctx, planet, browser, "/?promo=badCode")

		couponText := page.MustElement(".coupon-area__container__text-container").MustText()
		require.Contains(t, couponText, "Add a Coupon to Get Started")
		require.Contains(t, couponText, "Your coupon will show up here.")

	})
}
