// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"time"

	"github.com/araddon/dateparse"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/macaroon"
)

var shareCfg struct {
	APIKey            string   `help:"the api key to use for the satellite"`
	DisallowReads     bool     `default:"false" help:"if true, disallow reads"`
	DisallowWrites    bool     `default:"false" help:"if true, disallow writes"`
	DisallowLists     bool     `default:"false" help:"if true, disallow lists"`
	DisallowDeletes   bool     `default:"false" help:"if true, disallow deletes"`
	Readonly          bool     `default:"false" help:"implies disallow_writes and disallow_deletes"`
	Writeonly         bool     `default:"false" help:"implies disallow_reads and disallow_lists"`
	NotBefore         string   `help:"disallow access before this time"`
	NotAfter          string   `help:"disallow access after this time"`
	AllowedPathPrefix []string `help:"whitelist of bucket path prefixes to require"`
}

var shareRequiredPathPrefixes []string

func init() {
	shareCmd := &cobra.Command{
		Use:   "share",
		Short: "Creates a possibly restricted api key",
		RunE:  shareMain,
	}

	// We skip using addCmd like the other commands because it includes many
	// flags that aren't necessary for sharing, making the help text very
	// hard to understand.

	RootCmd.AddCommand(shareCmd)
	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}
	cfgstruct.Bind(shareCmd.Flags(), &shareCfg, defaults, cfgstruct.ConfDir(defaultConfDir))
}

func parseHumanDate(date string, now time.Time) (*time.Time, error) {
	if date == "" {
		return nil, nil
	} else if date == "now" {
		return &now, nil
	} else if date[0] == '+' {
		d, err := time.ParseDuration(date[1:])
		t := now.Add(d)
		return &t, errs.Wrap(err)
	} else if date[0] == '-' {
		d, err := time.ParseDuration(date[1:])
		t := now.Add(-d)
		return &t, errs.Wrap(err)
	} else {
		t, err := dateparse.ParseAny(date)
		return &t, errs.Wrap(err)
	}
}

// shareMain is the function executed when shareCmd is called
func shareMain(cmd *cobra.Command, args []string) (err error) {
	now := time.Now()

	notBefore, err := parseHumanDate(shareCfg.NotBefore, now)
	if err != nil {
		return err
	}
	notAfter, err := parseHumanDate(shareCfg.NotAfter, now)
	if err != nil {
		return err
	}

	// TODO(jeff): we have to have the server side of things expecting macaroons
	// before we can change libuplink to use macaroons because of all the tests.
	// For now, just use the raw macaroon library.

	key, err := macaroon.ParseAPIKey(shareCfg.APIKey)
	if err != nil {
		return err
	}

	caveat := macaroon.NewCaveat()
	caveat.DisallowDeletes = shareCfg.DisallowDeletes || shareCfg.Readonly
	caveat.DisallowLists = shareCfg.DisallowLists || shareCfg.Writeonly
	caveat.DisallowReads = shareCfg.DisallowReads || shareCfg.Writeonly
	caveat.DisallowWrites = shareCfg.DisallowWrites || shareCfg.Readonly
	caveat.NotBefore = notBefore
	caveat.NotAfter = notAfter

	for _, path := range shareCfg.AllowedPathPrefix {
		p, err := fpath.New(path)
		if err != nil {
			return err
		}
		if p.IsLocal() {
			return errs.New("required path must be remote: %q", path)
		}

		// TODO(jeff): The path should be encrypted somehow. This function
		//     encryption.EncryptPath(path, cipher, key)
		// should do the trick, but we need to figure out the cipher and key
		// to pass. The key should be local to the user, but the cipher
		// apparently depends on the bucket metadata.

		caveat.AllowedPaths = append(caveat.AllowedPaths, &macaroon.Caveat_Path{
			Bucket:              []byte(p.Bucket()),
			EncryptedPathPrefix: []byte(p.Path()),
		})
	}

	key, err = key.Restrict(caveat)
	if err != nil {
		return err
	}

	fmt.Println("new key:", key.Serialize())
	return nil
}
