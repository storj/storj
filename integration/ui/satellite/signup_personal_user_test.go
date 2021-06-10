package satellite

import (
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/stretchr/testify/require"
	"storj.io/common/testcontext"
	"storj.io/storj/integration/ui/uitest"
	"storj.io/storj/private/testplanet"
	"testing"
	"time"
)

func TestPersonalUserCanSignUp(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		signupPageURL := planet.Satellites[0].ConsoleURL() + "/signup"
		fullName := "John Doe"
		emailAddress := "test@email.com"
		password := "qazwsx123"
		page := browser.Timeout(10 * time.Second).MustPage(signupPageURL)
		page.MustSetViewport(1350, 600, 1, false)
		//First time User signup
		page.MustElement(".headerless-input").MustInput(fullName)
		page.MustElementX("//body/div[@id='app']/div[1]/div[2]/div[1]/div[4]/div[1]/input[1]").MustInput(emailAddress)
		page.MustElementX("//body/div[@id='app']/div[1]/div[2]/div[1]/div[5]/div[1]/div[1]/input[1]").MustInput(password)
		page.MustElementX("//body/div[@id='app']/div[1]/div[2]/div[1]/div[6]/div[1]/input[1]").MustInput(password)
		page.MustElementX("//body/div[@id='app']/div[1]/div[2]/div[1]/div[7]/label[1]/span[1]").MustClick()
		page.Keyboard.MustPress(input.Enter)
		confirmAccountEmailMessage := page.MustElement("div.register-area div.register-area__content-area div.register-success-area div.register-success-area__form-container > h2.register-success-area__form-container__title:nth-child(2)").MustText()
		require.Contains(t, confirmAccountEmailMessage,"You're almost there!")
	})
}