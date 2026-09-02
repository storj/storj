// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/geoip"
)

// PrepareCmdConfig configures the invoice prepare command.
type PrepareCmdConfig struct {
	ForceMandatory  bool     `help:"Force mandatory fields in invoices"`
	GeoIpDbs        []string `help:"GeoIP databases to use for IP address lookup" default:"GeoLite2-City.mmdb"`
	SkipOFAC        bool     `help:"Skip OFAC checks"`
	AllowUnscreened bool     `help:"Write payouts even if some nodes could not be OFAC-screened"`
	Prepayment      bool     `help:"Pay a prepayment on top of what the period owes: 90 percent of the at-rest compensation plus half of the egress compensation, scaled by 30/36. The period records it as paid as well as paid out, so it is not recovered from the next payout. Nodes that are disqualified, gracefully exited or exiting, offline, without a 1099, sanctioned, or still in withholding earn none. Invoices produced at a surge percent below 100 are rejected, since the prepayment is calculated from the pre-surge compensation"`
	Invoice         string   `help:"Path to the invoices CSV" required:"true"`
}

// Prepare is the command that turns invoices into incomplete paystubs and prepayouts.
type Prepare struct {
	log *zap.Logger
	cfg *PrepareCmdConfig
}

// NewPrepare constructs a Prepare command with the provided config.
func NewPrepare(log *zap.Logger, cfg *PrepareCmdConfig) *Prepare {
	return &Prepare{log: log, cfg: cfg}
}

// Run executes the prepare command.
func (p *Prepare) Run() (err error) {
	log := p.log
	if log == nil {
		log = zap.NewNop()
	}

	var geoIPDBs []*geoip.MaxmindDB
	defer func() {
		for _, geoIPDB := range geoIPDBs {
			err = errs.Combine(err, geoIPDB.Close())
		}
	}()
	if !p.cfg.SkipOFAC {
		if len(p.cfg.GeoIpDbs) == 0 {
			return errs.New("at least one GeoIP database must be configured")
		}
		for _, geoIPDBPath := range p.cfg.GeoIpDbs {
			if strings.Contains(geoIPDBPath, "-Country") {
				return errs.New("geo ip database looks like a country database, but a city level db is now required")
			}
			geoIPDB, err := geoip.OpenMaxmindDB(geoIPDBPath)
			if err != nil {
				return err
			}
			geoIPDBs = append(geoIPDBs, geoIPDB)
		}
	}

	invoicesIn, err := os.Open(p.cfg.Invoice)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, invoicesIn.Close()) }()

	ipaystubsPath := makeCSVPath(p.cfg.Invoice, "incompletepaystubs")
	prepayoutsPath := makeCSVPath(p.cfg.Invoice, "prepayouts")

	ipaystubsTmp, err := os.Create(ipaystubsPath + ".tmp")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		_ = ipaystubsTmp.Close()
		_ = os.Remove(ipaystubsTmp.Name())
	}()

	prepayoutsTmp, err := os.Create(prepayoutsPath + ".tmp")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		_ = prepayoutsTmp.Close()
		_ = os.Remove(prepayoutsTmp.Name())
	}()

	if err := compensation.Prepare(invoicesIn, ipaystubsTmp, prepayoutsTmp, compensation.PrepareConfig{
		ForceMandatory:  p.cfg.ForceMandatory,
		GeoIPDBs:        geoIPDBs,
		SkipOFAC:        p.cfg.SkipOFAC,
		AllowUnscreened: p.cfg.AllowUnscreened,
		Prepayment:      p.cfg.Prepayment,
		Log:             log,
	}); err != nil {
		return err
	}

	if err := ipaystubsTmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(ipaystubsTmp.Name(), ipaystubsPath); err != nil {
		return err
	}

	if err := prepayoutsTmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(prepayoutsTmp.Name(), prepayoutsPath); err != nil {
		return err
	}

	return nil
}

func makeCSVPath(in, out string) string {
	ext := filepath.Ext(in)
	dir, name := filepath.Split(in[:len(in)-len(ext)])

	out = strings.ToLower(out)
	name = strings.ToLower(name)

	var segments []string
	for _, segment := range strings.Split(name, "-") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}

	// remove the last element
	if len(segments) > 0 {
		segments = segments[:len(segments)-1]
	}

	segments = append(segments, out)
	return dir + strings.Join(segments, "-") + ext
}
