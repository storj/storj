// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"gopkg.in/yaml.v2"
)

// modularSatelliteExecutable is the name of the modular satellite binary (built from ./satellite/satellite).
const modularSatelliteExecutable = "satellite-modular"

// modularSatelliteComponents is the --components selection of every modular satellite process: the stripe mock
// replaces the real client, which needs an API key.
const modularSatelliteComponents = "stripe.Client=*stripe.mockStripeClient"

// modularSatelliteParams are the per-satellite values of the modular satellite configuration.
type modularSatelliteParams struct {
	Address        string
	PrivateAddress string
	ConsoleAddress string
	AdminAddress   string

	SatelliteDB string
	MetabaseDB  string

	RedisAddress string
	RedisStartDB int

	JobqNodeURL string

	MailTemplatePath string
	ConsoleStaticDir string
	AdminStaticDir   string
}

// modularSatelliteConfig returns the configuration of the modular satellite. The modular satellite has no
// `setup` command and no dev defaults (--defaults dev), therefore everything a local network needs is listed here.
func modularSatelliteConfig(p modularSatelliteParams) map[string]string {
	return map[string]string{
		"log.level": "debug",
		// the processes get their own --debug.addr on the command line, this is the fallback for `migrate`
		// (which rejects the flag) and prevents a release config.yaml from sharing one debug port
		"debug.addr": "127.0.0.1:0",
		// the modular console always starts the health check server, its default (localhost:10500) would collide
		// between parallel storj-sim networks
		"healthcheck.address": "127.0.0.1:0",

		// listen addresses: server.Config is registered as "server2" in the modular satellite, tlsopts as "server"
		"server2.address":              p.Address,
		"server2.private-address":      p.PrivateAddress,
		"server.revocation-dburl":      "redis://" + p.RedisAddress + "?db=" + strconv.Itoa(p.RedisStartDB+1),
		"server.extensions.revocation": "false",
		"server.use-peer-ca-whitelist": "false",

		// databases
		"database-options.url":   p.SatelliteDB,
		"metainfo.database-url":  p.MetabaseDB,
		"orders.encryption-keys": "0100000000000000=0100000000000000000000000000000000000000000000000000000000000000",

		"live-accounting.storage-backend": "redis://" + p.RedisAddress + "?db=" + strconv.Itoa(p.RedisStartDB),

		// repair queue
		"jobq.server-node-url":           p.JobqNodeURL,
		"jobq.tls.use-peer-ca-whitelist": "false",
		"jobq.tls.extensions.revocation": "false",

		// web console (separate `console` process) and admin
		"console.address":                        p.ConsoleAddress,
		"console.static-dir":                     p.ConsoleStaticDir,
		"console.auth-token-secret":              "my-suppa-secret-key",
		"console.open-registration-enabled":      "true",
		"console.rate-limit.burst":               "100",
		"console.signup-activation-code-enabled": "false",
		"admin.address":                          p.AdminAddress,
		"admin.static-dir":                       p.AdminStaticDir,

		// the modular console always initializes the KMS service (the classic api skipped it without key-infos),
		// the mock client derives the key from the version name and needs no credentials
		"key-management.mock-client": "true",
		"key-management.key-infos":   "1:secretversion1,12345",

		"mail.smtp-server-address": "smtp.gmail.com:587",
		"mail.from":                "Storj <yaroslav-satellite-test@storj.io>",
		"mail.template-path":       p.MailTemplatePath,
		"mail.auth-type":           "simulate",

		// a local network: every node has the same (private) IP, low difficulty identities, nobody is vetted
		"contact.allow-private-ip":               "true",
		"contact.rate-limit-interval":            "1ns",
		"contact.rate-limit-burst":               "1000",
		"overlay.node.distinct-ip":               "false",
		"overlay.minimum-new-node-id-difficulty": "0",
		"overlay.node.new-node-fraction":         "1",
		"reputation.audit-count":                 "0",
		"reputation.minimum-node-age":            "0h",
		"metainfo.rs":                            "4/6/8/10-256 B",

		// the classic sim ran garbage collection with the dev default (disabled)
		"garbage-collection.enabled": "false",
	}
}

// writeFlatYAML writes the config as flat `key: value` lines, the same shape as the config.yaml of `satellite setup`.
func writeFlatYAML(path string, config map[string]string) error {
	var content strings.Builder
	for _, key := range sortedKeys(config) {
		line, err := yamlLine(key, config[key])
		if err != nil {
			return err
		}
		content.WriteString(line)
	}
	return errs.Wrap(os.WriteFile(path, []byte(content.String()), 0644))
}

// ensureModularConfig appends the missing keys of config to an existing config.yaml (created by `satellite setup`
// of a previous release). Existing keys and comments are kept as they are.
func ensureModularConfig(dir string, config map[string]string) error {
	path := filepath.Join(dir, "config.yaml")
	vip := viper.New()
	vip.SetConfigFile(path)
	if err := vip.ReadInConfig(); err != nil {
		return errs.Wrap(err)
	}

	missing := map[string]string{}
	for key, value := range config {
		if !vip.IsSet(key) {
			missing[key] = value
		}
	}
	if len(missing) == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.WriteString("\n# added by storj-sim for the modular satellite\n"); err != nil {
		return errs.Wrap(err)
	}
	for _, key := range sortedKeys(missing) {
		line, err := yamlLine(key, missing[key])
		if err != nil {
			return err
		}
		if _, err := file.WriteString(line); err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}

// yamlLine renders one `key: value` line, quoting the value when YAML requires it (e.g. "true", "0").
func yamlLine(key, value string) (string, error) {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return "", errs.Wrap(err)
	}
	return key + ": " + strings.TrimSpace(string(encoded)) + "\n", nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
