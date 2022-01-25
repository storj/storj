// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/storj/cmd/uplinkng/ulloc"
	"storj.io/uplink"
)

const defaultAccessRegisterTimeout = 15 * time.Second

type cmdShare struct {
	ex ulext.External
	ap accessPermissions

	access      string
	exportTo    string
	baseURL     string
	register    bool
	url         bool
	dns         string
	authService string
	public      bool
}

func newCmdShare(ex ulext.External) *cmdShare {
	return &cmdShare{ex: ex}
}

func (c *cmdShare) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to share", "").(string)
	params.Break()

	c.exportTo = params.Flag("export-to", "Path to export the shared access to", "").(string)
	c.baseURL = params.Flag("base-url", "The base url for link sharing", "https://link.us1.storjshare.io").(string)
	c.register = params.Flag("register", "If true, creates and registers access grant", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.url = params.Flag("url", "If true, returns a url for the shared path. implies --register and --public", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.dns = params.Flag("dns", "Specify your custom hostname. if set, returns dns settings for web hosting. implies --register and --public", "").(string)
	c.authService = params.Flag("auth-service", "URL for shared auth service", "https://auth.us1.storjshare.io").(string)
	c.public = params.Flag("public", "If true, the access will be public. --dns and --url override this", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	params.Break()

	c.ap.Setup(params, false)
}

func (c *cmdShare) Execute(ctx clingy.Context) error {
	if len(c.ap.prefixes) == 0 {
		return errs.New("You must specify at least one prefix to share. Use the access restrict command to restrict with no prefixes.")
	}

	access, err := c.ex.OpenAccess(c.access)
	if err != nil {
		return err
	}

	access, err = c.ap.Apply(access)
	if err != nil {
		return err
	}

	isPublic := c.public || c.url || c.dns != ""

	if isPublic {
		if c.ap.notAfter.String() == "" {
			fmt.Fprintf(ctx, "It's not recommended to create a shared Access without an expiration date.")
			fmt.Fprintf(ctx, "If you wish to do so anyway, please run this command with --not-after=none.")
			return nil
		}

		if c.ap.notAfter.String() == "none" {
			c.ap.notAfter = time.Time{}
		}
	}

	newAccessData, err := access.Serialize()
	if err != nil {
		return err
	}

	fmt.Fprintf(ctx, "Sharing access to satellite %s\n", access.SatelliteAddress())
	fmt.Fprintf(ctx, "=========== ACCESS RESTRICTIONS ==========================================================\n")
	fmt.Fprintf(ctx, "Download  : %s\n", formatPermission(c.ap.AllowDownload()))
	fmt.Fprintf(ctx, "Upload    : %s\n", formatPermission(c.ap.AllowUpload()))
	fmt.Fprintf(ctx, "Lists     : %s\n", formatPermission(c.ap.AllowList()))
	fmt.Fprintf(ctx, "Deletes   : %s\n", formatPermission(c.ap.AllowDelete()))
	fmt.Fprintf(ctx, "NotBefore : %s\n", formatTimeRestriction(c.ap.notBefore))
	fmt.Fprintf(ctx, "NotAfter  : %s\n", formatTimeRestriction(c.ap.notAfter))
	fmt.Fprintf(ctx, "Paths     : %s\n", formatPaths(c.ap.prefixes))
	fmt.Fprintf(ctx, "=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========\n")
	fmt.Fprintf(ctx, "Access    : %s\n", newAccessData)

	if c.register || c.url || c.dns != "" {
		accessKey, secretKey, endpoint, err := RegisterAccess(ctx, access, c.authService, isPublic, defaultAccessRegisterTimeout)
		if err != nil {
			return err
		}
		err = DisplayGatewayCredentials(ctx, accessKey, secretKey, endpoint, "", "")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(ctx, "Public Access: ", isPublic)
		if err != nil {
			return err
		}

		if len(c.ap.prefixes) == 1 && !c.ap.AllowUpload() && !c.ap.disallowDeletes {
			if c.url {
				if err = createURL(ctx, accessKey, c.ap.prefixes[0], c.baseURL, c.ap.prefixes); err != nil {
					return err
				}
			}
			if c.dns != "" {
				if err = createDNS(ctx, accessKey, c.ap.prefixes[0], c.baseURL, c.dns); err != nil {
					return err
				}
			}
		}
	}

	if c.exportTo != "" {
		// convert to an absolute path, mostly for output purposes.
		exportTo, err := filepath.Abs(c.exportTo)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(exportTo, []byte(newAccessData+"\n"), 0600); err != nil {
			return err
		}
		fmt.Fprintln(ctx, "Exported to:", exportTo)
	}

	return nil
}

func formatPermission(allowed bool) string {
	if allowed {
		return "Allowed"
	}
	return "Disallowed"
}

func formatTimeRestriction(t time.Time) string {
	if t.IsZero() {
		return "No restriction"
	}
	return formatTime(true, t)
}

func formatPaths(sharePrefixes []uplink.SharePrefix) string {
	if len(sharePrefixes) == 0 {
		return "WARNING! The entire project is shared!"
	}

	var paths []string
	for _, prefix := range sharePrefixes {
		path := "sj://" + prefix.Bucket
		if len(prefix.Prefix) == 0 {
			path += "/ (entire bucket)"
		} else {
			path += "/" + prefix.Prefix
		}

		paths = append(paths, path)
	}

	return strings.Join(paths, "\n            ")
}

// RegisterAccess registers an access grant with a Gateway Authorization Service.
func RegisterAccess(ctx context.Context, access *uplink.Access, authService string, public bool, timeout time.Duration) (accessKey, secretKey, endpoint string, err error) {
	if authService == "" {
		return "", "", "", errs.New("no auth service address provided")
	}
	accessSerialized, err := access.Serialize()
	if err != nil {
		return "", "", "", errs.Wrap(err)
	}
	postData, err := json.Marshal(map[string]interface{}{
		"access_grant": accessSerialized,
		"public":       public,
	})
	if err != nil {
		return accessKey, "", "", errs.Wrap(err)
	}

	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/v1/access", authService), bytes.NewReader(postData))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	respBody := make(map[string]string)
	if err := json.Unmarshal(body, &respBody); err != nil {
		return "", "", "", errs.New("unexpected response from auth service: %s", string(body))
	}

	accessKey, ok := respBody["access_key_id"]
	if !ok {
		return "", "", "", errs.New("access_key_id missing in response")
	}
	secretKey, ok = respBody["secret_key"]
	if !ok {
		return "", "", "", errs.New("secret_key missing in response")
	}
	return accessKey, secretKey, respBody["endpoint"], nil
}

// Creates linksharing url for allowed path prefixes.
func createURL(ctx clingy.Context, newAccessData string, prefix uplink.SharePrefix, baseURL string, sharePrefixes []uplink.SharePrefix) (err error) {
	loc := ulloc.NewRemote(prefix.Bucket, prefix.Prefix)
	bucket, key, _ := loc.RemoteParts()

	fmt.Fprintf(ctx, "=========== BROWSER URL ==================================================================\n")
	fmt.Fprintf(ctx, "REMINDER  : Object key must end in '/' when trying to share recursively\n")
	fmt.Fprintf(ctx, "URL       : %s/s/%s/%s/%s\n", baseURL, url.PathEscape(newAccessData), bucket, key)

	return nil
}

// Creates dns record info for allowed path prefixes.
func createDNS(ctx clingy.Context, accessKey string, prefix uplink.SharePrefix, baseURL, dns string) (err error) {
	CNAME, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	rootString := ulloc.NewRemote(prefix.Bucket, prefix.Prefix).String()[5:]
	printStorjRoot := fmt.Sprintf("txt-%s\tIN\tTXT  \tstorj-root:%s", dns, rootString)

	fmt.Fprintf(ctx, "=========== DNS INFO =====================================================================\n")
	fmt.Fprintf(ctx, "Remember to update the $ORIGIN with your domain name. You may also change the $TTL.\n")
	fmt.Fprintf(ctx, "$ORIGIN example.com.\n")
	fmt.Fprintf(ctx, "$TTL    3600\n")
	fmt.Fprintf(ctx, "%s    \tIN\tCNAME\t%s.\n", dns, CNAME.Host)
	fmt.Fprintln(ctx, printStorjRoot)
	fmt.Fprintf(ctx, "txt-%s\tIN\tTXT  \tstorj-access:%s\n", dns, accessKey)

	return nil
}

// DisplayGatewayCredentials formats and writes credentials to stdout.
func DisplayGatewayCredentials(ctx clingy.Context, accessKey, secretKey, endpoint, format, awsProfile string) (err error) {
	switch format {
	case "env": // export / set compatible format
		// note that AWS_ENDPOINT configuration is not natively utilized by the AWS CLI
		_, err = fmt.Fprintf(ctx, "AWS_ACCESS_KEY_ID=%s\n"+
			"AWS_SECRET_ACCESS_KEY=%s\n"+
			"AWS_ENDPOINT=%s\n",
			accessKey, secretKey, endpoint)
		if err != nil {
			return err
		}
	case "aws": // aws configuration commands
		profile := ""
		if awsProfile != "" {
			profile = " --profile " + awsProfile
			_, err = fmt.Fprintf(ctx, "aws configure %s\n", profile)
			if err != nil {
				return err
			}
		}
		// note that the endpoint_url configuration is not natively utilized by the AWS CLI
		_, err = fmt.Fprintf(ctx, "aws configure %s set aws_access_key_id %s\n"+
			"aws configure %s set aws_secret_access_key %s\n"+
			"aws configure %s set s3.endpoint_url %s\n",
			profile, accessKey, profile, secretKey, profile, endpoint)
		if err != nil {
			return err
		}
	default: // plain text
		_, err = fmt.Fprintf(ctx, "========== CREDENTIALS ===================================================================\n"+
			"Access Key ID: %s\n"+
			"Secret Key   : %s\n"+
			"Endpoint     : %s\n",
			accessKey, secretKey, endpoint)
		if err != nil {
			return err
		}
	}
	return nil
}
