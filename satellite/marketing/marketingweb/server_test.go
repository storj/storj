package marketingweb_test

import (
	"net/http"
	"testing"
	"net/url"
	"strings"
	"log"

	"github.com/stretchr/testify/require"
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

func buildResources(endpoint string, isReferralOffer bool) (url.URL, url.Values, error){
	URL, err := url.ParseRequestURI("http://127.0.0.1:10003")
	if err != nil{
		log.Printf("URL parsing Err : %v\n", err)
		return *URL, url.Values{}, err
	}
	URL.Path = endpoint
	form := buildForm(isReferralOffer)
	URL.RawQuery = form.Encode()
	return *URL, form, nil
}

func callServer(t *testing.T,endpoint,params string, form url.Values) string{
	c := http.Client{}
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(params))
	require.NoError(t,err,"failed to create new POST request")

	req.PostForm = form
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Do(req)
	if err != nil {
		require.NoError(t,err,"failed to execute POST request.")
	}
	return resp.Status
}

func TestCreateFreeCredit(t *testing.T){
	URL,form,err := buildResources("/create/free-credit", false)
	require.NoError(t,err,"failed to build request resources")

	urlStr := URL.String()
	respStatus := callServer(t,urlStr,URL.RawQuery,form)

	if respStatus != "200 OK" {
		t.Fatalf("Bad http response status : %v\n", respStatus)
	}
}

func TestCreateReferralOffer(t *testing.T){
	URL,form,err := buildResources("/create/referral-offer", true)
	require.NoError(t,err,"failed to build request resources")

	urlStr := URL.String()
	respStatus := callServer(t,urlStr,URL.RawQuery,form)

	if respStatus != "200 OK" {
		t.Fatalf("Bad http response status : %v\n", respStatus)
	}
}

func TestGetOffers(t *testing.T){
	url := "http://127.0.0.1:10003/"
	resp, err := http.Get(url)
	require.NoError(t,err,"failed to execute GET request.")

	defer resp.Body.Close()

	if resp.Status != "200 OK" {
		t.Fatalf("Bad http response status : %v\n", resp.Status)
	}
}