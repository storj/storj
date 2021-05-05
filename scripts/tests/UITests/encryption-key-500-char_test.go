package UITests

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func Test_500EncryptionKey(t *testing.T) {

	var encryptionPassphrase500 = "YstOoAderrEGEUSlcd8CaORnB5Igjw3Fp4LIpsDbikhIljqb2k0JGgLlq2mXXErkglYspz9xpNPvWC4WvQQAHe2PCwZo52vhc3rgJJjzQuy4zipOHdMLqgJiyiSppQBMAiAK1llWjh5UGAr8gkBOAr0OFCFPHIGVx1UR3hPA7hbVXm5RHbhHRV17ooEnCCJMQ7AIgzadV71M5JKBVFWSTffbe2dJMVbE9rqjrEQx5JiDaGo18Le8NDbYdFSlgl6LoB9uZFEhH9XoVyWvvY3UTvjhfxQKJ2iV8eDXR1FWbTFGtSnMEO4PiUD5fvSOrZkaCxXwE6rNSPPFLj6npBgu3nxtilonY66PamfuHsoQvbWp2RFdhu2lcgPlTBn3Elv4KOZwB7VFuCa4FfEN18uQGJhaMST2rVLG8CGtV3ulmxVirgLofbM1hlqllLYFf6Ex0qsakuWEXwHy2qIAJQGQRN1HATzYoAnQu04pYsLnMKTzS8aXqTZ3"

	page, browser := login_to_account()
	//check title
	fmt.Println(strings.Contains(page.MustElement(".dashboard-area__title").MustText(), "Dashboard"))
	// Output: true

	//select objects on left
	page.MustElement("a.navigation-area__item-container:nth-of-type(2)").MustClick()

	//object browser warning is shown click continue
	page.MustElementX("//span[contains(text(),'Continue')]").MustClick()

	//encryption passphrase is shown, click on enter phrase for custom encryption key
	page.MustElementX("//p[contains(text(),'Enter Phrase')]").MustClick()

	//encryption key with 500 char is used for new custom encryption key
	page.MustElementX("//input[@id='']").MustInput(encryptionPassphrase500)
	page.MustElementX("//body/div[@id='app']/div[1]/div[1]/div[2]/div[2]/div[2]/div[1]/div[1]/div[1]/div[4]").MustClick()

	//new encryption key with 500 char is used to access data
	page.MustElementX("//textarea[@id='enter-pass-textarea']").MustInput(encryptionPassphrase500)
	page.MustElementX("//body/div[@id='app']/div[1]/div[1]/div[2]/div[2]/div[2]/div[1]/div[1]/div[2]").MustClick()

	//whether using generated or custom encryption key it loads the bucket page indefinitely
	time.Sleep(5*time.Second)

	defer browser.MustClose()
}