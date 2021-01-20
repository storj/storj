// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/uplink"
)

var shareCfg struct {
	DisallowReads     bool     `default:"false" help:"if true, disallow reads" basic-help:"true"`
	DisallowWrites    bool     `default:"false" help:"if true, disallow writes. see also --readonly" basic-help:"true"`
	DisallowLists     bool     `default:"false" help:"if true, disallow lists" basic-help:"true"`
	DisallowDeletes   bool     `default:"false" help:"if true, disallow deletes. see also --readonly" basic-help:"true"`
	Readonly          bool     `default:"true" help:"implies --disallow-writes and --disallow-deletes. you must specify --readonly=false if you don't want this" basic-help:"true"`
	Writeonly         bool     `default:"false" help:"implies --disallow-reads and --disallow-lists" basic-help:"true"`
	NotBefore         string   `help:"disallow access before this time (e.g. '+2h', '2020-01-02T15:01:01-01:00')" basic-help:"true"`
	NotAfter          string   `help:"disallow access after this time (e.g. '+2h', '2020-01-02T15:01:01-01:00')" basic-help:"true"`
	AllowedPathPrefix []string `help:"whitelist of path prefixes to require, overrides the [allowed-path-prefix] arguments"`
	ExportTo          string   `default:"" help:"path to export the shared access to" basic-help:"true"`
	BaseURL           string   `default:"https://link.tardigradeshare.io" help:"the base url for link sharing" basic-help:"true"`

	Register    bool   `default:"false" help:"if true, creates and registers access grant" basic-help:"true"`
	URL         bool   `default:"false" help:"if true, returns a url for the shared path. implies --register and --public" basic-help:"true"`
	DNS         string `default:"" help:"specify your custom hostname. if set, returns dns settings for web hosting. implies --register and --public" basic-help:"true"`
	AuthService string `default:"https://auth.tardigradeshare.io" help:"url for shared auth service" basic-help:"true"`
	Public      bool   `default:"false" help:"if true, the access will be public. --dns and --url override this" basic-help:"true"`

	// Share requires information about the current access
	AccessConfig
}

func init() {
	// We skip the use of addCmd here because we only want the configuration options listed
	// above, and addCmd adds a whole lot more than we want.

	shareCmd := &cobra.Command{
		Use:   "share [ALLOWED_PATH_PREFIX]...",
		Short: "Shares restricted access to objects.",
		RunE:  shareMain,
	}

	RootCmd.AddCommand(shareCmd)

	process.Bind(shareCmd, &shareCfg, defaults, cfgstruct.ConfDir(getConfDir()))
}

func shareMain(cmd *cobra.Command, args []string) (err error) {
	newAccess, newAccessData, sharePrefixes, permission, err := createAccessGrant(args)
	if err != nil {
		return err
	}

	var accessKey string

	if shareCfg.Register || shareCfg.URL || shareCfg.DNS != "" {
		isPublic := (shareCfg.Public || shareCfg.URL || shareCfg.DNS != "")
		accessKey, _, _, err = RegisterAccess(newAccess, shareCfg.AuthService, isPublic, defaultAccessRegisterTimeout)
		if err != nil {
			return err
		}
		fmt.Println("Public Access: ", isPublic)

		if len(shareCfg.AllowedPathPrefix) == 1 && !permission.AllowUpload && !permission.AllowDelete {
			if shareCfg.URL {
				if err = createURL(accessKey, sharePrefixes); err != nil {
					return err
				}
			}
			if shareCfg.DNS != "" {
				if err = createDNS(accessKey); err != nil {
					return err
				}
			}
		}
	}

	if shareCfg.ExportTo != "" {
		// convert to an absolute path, mostly for output purposes.
		exportTo, err := filepath.Abs(shareCfg.ExportTo)
		if err != nil {
			return Error.Wrap(err)
		}
		if err := ioutil.WriteFile(exportTo, []byte(newAccessData+"\n"), 0600); err != nil {
			return Error.Wrap(err)
		}
		fmt.Println("Exported to:", exportTo)
	}
	return nil
}

// Creates access grant for allowed path prefixes.
func createAccessGrant(args []string) (newAccess *uplink.Access, newAccessData string, sharePrefixes []sharePrefixExtension, permission uplink.Permission, err error) {
	now := time.Now()
	notBefore, err := parseHumanDate(shareCfg.NotBefore, now)
	if err != nil {
		return newAccess, newAccessData, sharePrefixes, permission, err
	}
	notAfter, err := parseHumanDate(shareCfg.NotAfter, now)
	if err != nil {
		return newAccess, newAccessData, sharePrefixes, permission, err
	}

	if len(shareCfg.AllowedPathPrefix) == 0 {
		// if the --allowed-path-prefix flag is not set,
		// use any arguments as allowed path prefixes
		for _, arg := range args {
			shareCfg.AllowedPathPrefix = append(shareCfg.AllowedPathPrefix, strings.Split(arg, ",")...)
		}
	}

	var uplinkSharePrefixes []uplink.SharePrefix
	for _, path := range shareCfg.AllowedPathPrefix {
		p, err := fpath.New(path)
		if err != nil {
			return newAccess, newAccessData, sharePrefixes, permission, err
		}
		if p.IsLocal() {
			return newAccess, newAccessData, sharePrefixes, permission, errs.New("required path must be remote: %q", path)
		}

		uplinkSharePrefix := uplink.SharePrefix{
			Bucket: p.Bucket(),
			Prefix: p.Path(),
		}
		sharePrefixes = append(sharePrefixes, sharePrefixExtension{
			uplinkSharePrefix: uplinkSharePrefix,
			hasTrailingSlash:  strings.HasSuffix(path, "/"),
		})
		uplinkSharePrefixes = append(uplinkSharePrefixes, uplinkSharePrefix)
	}

	access, err := shareCfg.GetAccess()
	if err != nil {
		return newAccess, newAccessData, sharePrefixes, permission, err
	}

	permission = uplink.Permission{}
	permission.AllowDelete = !shareCfg.DisallowDeletes && !shareCfg.Readonly
	permission.AllowList = !shareCfg.DisallowLists && !shareCfg.Writeonly
	permission.AllowDownload = !shareCfg.DisallowReads && !shareCfg.Writeonly
	permission.AllowUpload = !shareCfg.DisallowWrites && !shareCfg.Readonly
	permission.NotBefore = notBefore
	permission.NotAfter = notAfter

	newAccess, err = access.Share(permission, uplinkSharePrefixes...)
	if err != nil {
		return newAccess, newAccessData, sharePrefixes, permission, err
	}

	newAccessData, err = newAccess.Serialize()
	if err != nil {
		return newAccess, newAccessData, sharePrefixes, permission, err
	}

	satelliteAddr, _, _, err := parseAccess(newAccessData)
	if err != nil {
		return newAccess, newAccessData, sharePrefixes, permission, err
	}

	fmt.Println("Sharing access to satellite", satelliteAddr)
	fmt.Println("=========== ACCESS RESTRICTIONS ==========================================================")
	fmt.Println("Download  :", formatPermission(permission.AllowDownload))
	fmt.Println("Upload    :", formatPermission(permission.AllowUpload))
	fmt.Println("Lists     :", formatPermission(permission.AllowList))
	fmt.Println("Deletes   :", formatPermission(permission.AllowDelete))
	fmt.Println("NotBefore :", formatTimeRestriction(permission.NotBefore))
	fmt.Println("NotAfter  :", formatTimeRestriction(permission.NotAfter))
	fmt.Println("Paths     :", formatPaths(sharePrefixes))
	fmt.Println("=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========")
	fmt.Println("Access    :", newAccessData)

	return newAccess, newAccessData, sharePrefixes, permission, nil
}

// Creates linksharing url for allowed path prefixes.
func createURL(newAccessData string, sharePrefixes []sharePrefixExtension) (err error) {
	p, err := fpath.New(shareCfg.AllowedPathPrefix[0])
	if err != nil {
		return err
	}
	fmt.Println("=========== BROWSER URL ==================================================================")
	fmt.Println("REMINDER  : Object key must end in '/' when trying to share recursively")

	var printFormat string
	if p.Path() == "" || !sharePrefixes[0].hasTrailingSlash { // Check if the path is empty (aka sharing the entire bucket) or the path is not a directory or an object that ends in "/".
		printFormat = "URL       : %s/%s/%s/%s\n"
	} else {
		printFormat = "URL       : %s/%s/%s/%s/\n"
	}
	fmt.Printf(printFormat, shareCfg.BaseURL, url.PathEscape(newAccessData), p.Bucket(), p.Path())
	return nil
}

// Creates dns record info for allowed path prefixes.
func createDNS(accessKey string) (err error) {
	p, err := fpath.New(shareCfg.AllowedPathPrefix[0])
	if err != nil {
		return err
	}
	CNAME, err := url.Parse(shareCfg.BaseURL)
	if err != nil {
		return err
	}

	minWidth := len(shareCfg.DNS) + 5 // add 5 spaces to account for "txt-"
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, minWidth, minWidth, 0, '\t', 0)
	defer func() {
		err = errs.Combine(err, w.Flush())
	}()

	var printStorjRoot string
	if p.Path() == "" {
		printStorjRoot = fmt.Sprintf("txt-%s\tIN\tTXT  \tstorj-root:%s", shareCfg.DNS, p.Bucket())
	} else {
		printStorjRoot = fmt.Sprintf("txt-%s\tIN\tTXT  \tstorj-root:%s/%s", shareCfg.DNS, p.Bucket(), p.Path())
	}

	fmt.Println("=========== DNS INFO =====================================================================")
	fmt.Println("Remember to update the $ORIGIN with your domain name. You may also change the $TTL.")
	fmt.Fprintln(w, "$ORIGIN example.com.")
	fmt.Fprintln(w, "$TTL    3600")
	fmt.Fprintf(w, "%s    \tIN\tCNAME\t%s.\n", shareCfg.DNS, CNAME.Host)
	fmt.Fprintln(w, printStorjRoot)
	fmt.Fprintf(w, "txt-%s\tIN\tTXT  \tstorj-access:%s\n", shareCfg.DNS, accessKey)

	return nil
}

func parseHumanDate(date string, now time.Time) (time.Time, error) {
	switch {
	case date == "":
		return time.Time{}, nil
	case date == "now":
		return now, nil
	case date[0] == '+':
		d, err := time.ParseDuration(date[1:])
		t := now.Add(d)
		return t, errs.Wrap(err)
	case date[0] == '-':
		d, err := time.ParseDuration(date[1:])
		t := now.Add(-d)
		return t, errs.Wrap(err)
	default:
		t, err := time.Parse(time.RFC3339, date)
		return t, errs.Wrap(err)
	}
}

// sharePrefixExtension is a temporary struct type. We might want to add hasTrailingSlash bool to `uplink.SharePrefix` directly.
type sharePrefixExtension struct {
	uplinkSharePrefix uplink.SharePrefix
	hasTrailingSlash  bool
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
	return formatTime(t)
}

func formatPaths(sharePrefixes []sharePrefixExtension) string {
	if len(sharePrefixes) == 0 {
		return "WARNING! The entire project is shared!"
	}

	var paths []string
	for _, prefix := range sharePrefixes {
		path := "sj://" + prefix.uplinkSharePrefix.Bucket
		if len(prefix.uplinkSharePrefix.Prefix) == 0 {
			path += "/ (entire bucket)"
		} else {
			path += "/" + prefix.uplinkSharePrefix.Prefix
			if prefix.hasTrailingSlash {
				path += "/"
			}
		}

		paths = append(paths, path)
	}

	return strings.Join(paths, "\n            ")
}
