package marketingweb

import (
	"net/http"
	"testing"
	"net/url"
	"strings"
	"log"
)


func buildForm(isReferralOffer bool) url.Values{
	form := url.Values{}
	form.Add("Name","May Credit")
	form.Add("Description","desc")
	form.Add("ExpiresAt","2019-06-27")
	form.Add("InviteeCreditInCents","50")
	form.Add("InviteeCreditDurationDays","50")
	form.Add("RedeemableCap","150")

	if isReferralOffer {
		form.Add("AwardCreditInCents","50")
		form.Add("AwardCreditDurationDays","50")
	}
	return form
}

func buildResources(endpoint string, isReferralOffer bool) (url.URL, url.Values){
	URL, err := url.ParseRequestURI("http://127.0.0.1:10003")
	if err != nil{
		log.Fatalf("URL parsing Err : %v\n", err)
	}
	URL.Path = endpoint
	form := buildForm(isReferralOffer)
	URL.RawQuery = form.Encode()
	return *URL, form
}

func callServer(t *testing.T,endpoint,params string, form url.Values) string{
	c := http.Client{}
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(params))
	if err != nil {
		t.Fatalf("Err building request : %v\n", err)
	}
	req.PostForm = form
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Resp Err : %v\n", err)
	}
	return resp.Status
}

func TestCreateFreeCredit(t *testing.T){
	URL,form := buildResources("/create/free-credit", false)
	urlStr := URL.String()
	respStatus := callServer(t,urlStr,URL.RawQuery,form)

	if respStatus != "200 OK" {
		t.Fatalf("response err : %v\n", respStatus)
	}
}

func TestCreateReferralOffer(t *testing.T){
	URL,form := buildResources("/create/referral-offer", true)
	urlStr := URL.String()
	respStatus := callServer(t,urlStr,URL.RawQuery,form)

	if respStatus != "200 OK" {
		t.Fatalf("response err : %v\n", respStatus)
	}
}

func TestGetOffers(t *testing.T){
	url := "http://127.0.0.1:10003/"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("request err : %v\n", err)
	}
	defer resp.Body.Close()

	if resp.Status != "200 OK" {
		t.Fatalf("response err : %v\n", err)
	}
}

func TestStopOffer(t *testing.T){
	url := "http://127.0.0.1:10003"
	req,err := http.NewRequest("PUT",url+"/stop/1",nil)
	if err != nil {
		t.Fatalf("create request err : %v\n", err)
	}

	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("response Err : %v\n", err)
	}

	if resp.Status != "200 OK"{
		t.Fatalf("bad status code : %v\n", resp.Status)
	}
}