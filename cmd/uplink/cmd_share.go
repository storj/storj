// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/uplink"
	"storj.io/uplink/edge"
	privateEdge "storj.io/uplink/private/edge"
)

type cmdShare struct {
	ex ulext.External
	ap accessPermissions

	access      string
	exportTo    string
	baseURL     string
	register    bool
	url         bool
	dns         string
	tls         bool
	authService string
	caCert      string
	public      bool
}

func newCmdShare(ex ulext.External) *cmdShare {
	return &cmdShare{ex: ex}
}

func (c *cmdShare) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to share", "").(string)
	params.Break()

	c.exportTo = params.Flag("export-to", "Path to export the shared access to", "").(string)
	c.baseURL = params.Flag("base-url", "The base URL for link sharing", "https://link.storjshare.io").(string)
	c.register = params.Flag("register", "If true, creates and registers access grant", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.url = params.Flag("url", "If true, returns a URL for the shared path. Implies --register and --public", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.dns = params.Flag("dns", "Specify your custom domain. If set, returns DNS settings for web hosting. Implies --register and --public", "").(string)
	c.tls = params.Flag("tls", "Return an additional TXT record to secure your domain (Pro Accounts only). Implies --dns and --public", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.authService = params.Flag("auth-service", "URL for shared auth service", "https://auth.storjshare.io").(string)
	c.public = params.Flag("public", "If true, the access will be public. --dns and --url override this", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	params.Break()

	c.ap.Setup(params, false)
}

func (c *cmdShare) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(c.ap.prefixes) == 0 {
		return errs.New("you must specify at least one prefix to share. Use the access restrict command to restrict with no prefixes")
	}

	access, err := c.ex.OpenAccess(c.access)
	if err != nil {
		return err
	}

	access, err = c.ap.Apply(access)
	if err != nil {
		return err
	}

	if c.tls && c.dns == "" {
		return errs.New("you must specify your custom domain with --dns")
	}

	c.public = c.public || c.url || c.dns != "" || c.tls

	if c.public {
		c.register = true

		if c.ap.notAfter == nil {
			_, _ = fmt.Fprintf(clingy.Stdout(ctx), "It's not recommended to create a shared Access without an expiration date.\n")
			_, _ = fmt.Fprintf(clingy.Stdout(ctx), "If you wish to do so anyway, please run this command with --not-after=none.\n")
			return nil
		}
	}

	newAccessData, err := access.Serialize()
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Sharing access to satellite %s\n", access.SatelliteAddress())
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "=========== ACCESS RESTRICTIONS ==========================================================\n")
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Download     : %s\n", formatPermission(c.ap.AllowDownload()))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Upload       : %s\n", formatPermission(c.ap.AllowUpload()))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Lists        : %s\n", formatPermission(c.ap.AllowList()))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Deletes      : %s\n", formatPermission(c.ap.AllowDelete()))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "NotBefore    : %s\n", formatTimeRestriction(c.ap.NotBefore()))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "NotAfter     : %s\n", formatTimeRestriction(c.ap.NotAfter()))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "MaxObjectTTL : %s\n", formatDuration(c.ap.maxObjectTTL))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Paths        : %s\n", formatPaths(c.ap.prefixes))
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========\n")
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Access       : %s\n", newAccessData)

	if c.register {
		info, err := c.ex.GetEdgeUrlOverrides(ctx, access)
		if err != nil {
			return errs.New("could not get project info: %w", err)
		}

		authService := c.authService
		linksharingUrl := c.baseURL
		if info.AuthService != "" {
			authService = info.AuthService
		}
		if info.PublicLinksharing != "" {
			linksharingUrl = info.PublicLinksharing
		}

		credentials, err := RegisterAccess(ctx, access, authService, c.public, c.caCert)
		if err != nil {
			return err
		}
		err = DisplayGatewayCredentials(ctx, *credentials, "", "")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(clingy.Stdout(ctx), "Public Access:", c.public)
		if err != nil {
			return err
		}

		if c.url {
			if c.ap.AllowUpload() || c.ap.AllowDelete() {
				return errs.New("will only generate linksharing URL with readonly restrictions")
			}

			err = createURL(ctx, credentials.AccessKeyID, c.ap.prefixes, linksharingUrl)
			if err != nil {
				return err
			}
		}

		if c.dns != "" {
			if c.ap.AllowUpload() || c.ap.AllowDelete() {
				return errs.New("will only generate DNS entries with readonly restrictions")
			}

			err = createDNS(ctx, credentials.AccessKeyID, c.ap.prefixes, linksharingUrl, c.dns, c.tls)
			if err != nil {
				return err
			}
		}
	}

	if c.exportTo != "" {
		// convert to an absolute path, mostly for output purposes.
		exportTo, err := filepath.Abs(c.exportTo)
		if err != nil {
			return err
		}
		// TODO: this should use the ulfs package so that tests can run without actually
		// writing files out.
		if err := os.WriteFile(exportTo, []byte(newAccessData+"\n"), 0600); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(clingy.Stdout(ctx), "Exported to:", exportTo)
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

func formatDuration(d *time.Duration) string {
	if d == nil {
		return "Not set"
	}
	return d.String()
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
func RegisterAccess(ctx context.Context, access *uplink.Access, authService string, public bool, certificateFile string) (credentials *privateEdge.Credentials, err error) {
	if authService == "" {
		return nil, errs.New("no auth service address provided")
	}

	var edgeConfig edge.Config

	if strings.HasPrefix(authService, "insecure://") {
		authService = strings.TrimPrefix(authService, "insecure://")
		edgeConfig.InsecureUnencryptedConnection = true
	}
	// preserve compatibility with previous https service
	authService = strings.TrimPrefix(authService, "https://")
	authService = strings.TrimSuffix(authService, "/")
	if !strings.Contains(authService, ":") {
		authService += ":7777"
	}

	var certificatePEM []byte
	if certificateFile != "" {
		certificatePEM, err = os.ReadFile(certificateFile)
		if err != nil {
			return nil, errs.New("can't read certificate file: %w", err)
		}
	}

	edgeConfig.AuthServiceAddress = authService
	edgeConfig.CertificatePEM = certificatePEM

	return privateEdge.RegisterAccess(ctx, &edgeConfig, access, &edge.RegisterAccessOptions{Public: public})
}

// Creates linksharing url for allowed path prefixes.
func createURL(ctx context.Context, accessKeyID string, prefixes []uplink.SharePrefix, baseURL string) (err error) {
	if len(prefixes) == 0 {
		return errs.New("need at least a bucket to create a working linkshare URL")
	}

	bucket := prefixes[0].Bucket
	key := prefixes[0].Prefix

	url, err := edge.JoinShareURL(baseURL, accessKeyID, bucket, key, nil)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "=========== BROWSER URL ==================================================================\n")
	if key != "" && key[len(key)-1:] != "/" {
		_, _ = fmt.Fprintf(clingy.Stdout(ctx), "REMINDER  : Object key must end in '/' when trying to share a prefix\n")
	}
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "URL       : %s\n", url)
	return nil
}

// Creates dns record info for allowed path prefixes.
func createDNS(ctx context.Context, accessKey string, prefixes []uplink.SharePrefix, baseURL, dns string, tls bool) (err error) {
	if len(prefixes) == 0 {
		return errs.New("need at least a bucket to create DNS records")
	}

	bucket := prefixes[0].Bucket
	key := prefixes[0].Prefix

	CNAME, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	var printStorjRoot string
	if key == "" {
		printStorjRoot = fmt.Sprintf("txt-%s\tIN\tTXT  \tstorj-root:%s", dns, bucket)
	} else {
		printStorjRoot = fmt.Sprintf("txt-%s\tIN\tTXT  \tstorj-root:%s/%s", dns, bucket, key)
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "=========== DNS INFO =====================================================================\n")
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Remember to update the $ORIGIN with your domain name. You may also change the $TTL.\n")
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "$ORIGIN example.com.\n")
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "$TTL    3600\n")
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "%s    \tIN\tCNAME\t%s.\n", dns, CNAME.Host)
	_, _ = fmt.Fprintln(clingy.Stdout(ctx), printStorjRoot)
	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "txt-%s\tIN\tTXT  \tstorj-access:%s\n", dns, accessKey)
	if tls {
		_, _ = fmt.Fprintf(clingy.Stdout(ctx), "txt-%s\tIN\tTXT  \tstorj-tls:true\n", dns)
	}

	return nil
}

// DisplayGatewayCredentials formats and writes credentials to stdout.
func DisplayGatewayCredentials(ctx context.Context, credentials privateEdge.Credentials, format string, awsProfile string) (err error) {
	switch format {
	case "env": // export / set compatible format
		err = printExpirationComment(clingy.Stdout(ctx), credentials.FreeTierRestrictedExpiration)
		if err != nil {
			return err
		}
		// note that AWS_ENDPOINT configuration is not natively utilized by the AWS CLI
		_, err = fmt.Fprintf(clingy.Stdout(ctx), "AWS_ACCESS_KEY_ID=%s\n"+
			"AWS_SECRET_ACCESS_KEY=%s\n"+
			"AWS_ENDPOINT=%s\n",
			credentials.AccessKeyID, credentials.SecretKey, credentials.Endpoint)
		if err != nil {
			return err
		}
	case "aws": // aws configuration commands
		err = printExpirationComment(clingy.Stdout(ctx), credentials.FreeTierRestrictedExpiration)
		if err != nil {
			return err
		}
		profile := ""
		if awsProfile != "" {
			profile = " --profile " + awsProfile
			_, err = fmt.Fprintf(clingy.Stdout(ctx), "aws configure %s\n", profile)
			if err != nil {
				return err
			}
		}
		// note that the endpoint_url configuration is not natively utilized by the AWS CLI
		_, err = fmt.Fprintf(clingy.Stdout(ctx), "aws configure %s set aws_access_key_id %s\n"+
			"aws configure %s set aws_secret_access_key %s\n"+
			"aws configure %s set s3.endpoint_url %s\n",
			profile, credentials.AccessKeyID, profile, credentials.SecretKey, profile, credentials.Endpoint)
		if err != nil {
			return err
		}
	case "om", "objectmount", "object-mount": // object mount compatible format
		_, err = fmt.Fprintf(clingy.Stdout(ctx), "aws_access_key_id = %s\n"+
			"aws_secret_access_key = %s\n"+
			"endpoint = %s\n",
			credentials.AccessKeyID, credentials.SecretKey, credentials.Endpoint)
		if err != nil {
			return err
		}
	default: // plain text
		_, err = fmt.Fprintln(clingy.Stdout(ctx), "========== GATEWAY CREDENTIALS ===========================================================")
		if err != nil {
			return err
		}

		if credentials.FreeTierRestrictedExpiration != nil {
			_, err = fmt.Fprintf(clingy.Stdout(ctx),
				"Trial account credentials automatically expire.\n"+
					"Expiration   : %s\n",
				formatTime(true, *credentials.FreeTierRestrictedExpiration))
			if err != nil {
				return err
			}
		}

		_, err = fmt.Fprintf(clingy.Stdout(ctx),
			"Access Key ID: %s\n"+
				"Secret Key   : %s\n"+
				"Endpoint     : %s\n",
			credentials.AccessKeyID, credentials.SecretKey, credentials.Endpoint)
		if err != nil {
			return err
		}
	}
	return nil
}

func printExpirationComment(w io.Writer, expiration *time.Time) error {
	if expiration == nil {
		return nil
	}
	_, err := fmt.Fprintf(w, "# Your trial account credentials will expire at %s.\n", formatTime(true, *expiration))
	return err
}
