// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"gopkg.in/yaml.v2"
)

type VersionsTest struct {
	Stage1 *Stage `yaml:"stage1"`
	Stage2 *Stage `yaml:"stage2"`
}

type Stage struct {
	SatelliteVersion    string   `yaml:"sat_version"`
	UplinkVersion       string   `yaml:"uplink_version"`
	StoragenodeVersions []string `yaml:"storagenode_versions"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) < 3 {
		return errors.New("Please provide path to script file and yaml file via command line")
	}

	scriptFile := os.Args[1]
	yamlFile := os.Args[2]

	b, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		return err
	}

	var tests []*VersionsTest
	if err := yaml.Unmarshal(b, &tests); err != nil {
		return err
	}

	for _, test := range tests {
		if err := runTest(test, scriptFile); err != nil {
			return err
		}
	}

	return nil
}

func runTest(test *VersionsTest, scriptFile string) error {
	stage1SNVersions := formatSNVersions(test.Stage1.StoragenodeVersions)
	stage2SNVersions := formatSNVersions(test.Stage2.StoragenodeVersions)
	cmd := exec.Command(scriptFile, test.Stage1.SatelliteVersion, test.Stage1.UplinkVersion, stage1SNVersions, test.Stage2.SatelliteVersion, test.Stage2.UplinkVersion, stage2SNVersions)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func formatSNVersions(snvs []string) string {
	var s string
	for i, snv := range snvs {
		space := " "
		if i == 0 {
			space = ""
		}
		s = fmt.Sprintf("%s%s%s", s, space, snv)
	}
	return s
}
