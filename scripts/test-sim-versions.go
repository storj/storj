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
	"strings"

	"gopkg.in/yaml.v2"
)

var SkippedVersions = []string{
	"v0.10.1",
	"v0.10.0", "v0.10.2",
	"v0.11.0", "v0.11.1", "v0.11.2", "v0.11.3", "v0.11.4", "v0.11.5", "v0.11.6", "v0.11.7",
	"v0.12.0", "v0.12.1", "v0.12.2", "v0.12.3", "v0.12.4", "v0.12.5", "v0.12.6",
	"v0.13.0", "v0.13.1", "v0.13.2", "v0.13.3", "v0.13.4", "v0.13.5", "v0.13.6",
}

type VersionsTest struct {
	Stage1 *Stage `yaml:"stage1"`
	Stage2 *Stage `yaml:"stage2"`
}

type Stage struct {
	SatelliteVersion    string   `yaml:"sat_version"`
	UplinkVersions      []string `yaml:"uplink_versions"`
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

	var filteredTagList []string
	for _, test := range tests {
		filteredTagList, err = getVersions(test, filteredTagList)
		if err != nil {
			return err
		}
		if len(test.Stage1.UplinkVersions) < 1 {
			test.Stage1.UplinkVersions = filteredTagList
		}
		if len(test.Stage2.UplinkVersions) < 1 {
			test.Stage2.UplinkVersions = filteredTagList
		}

		if err := runTest(test, scriptFile); err != nil {
			return err
		}
	}

	return nil
}

func runTest(test *VersionsTest, scriptFile string) error {
	stage1SNVersions := formatMultipleVersions(test.Stage1.StoragenodeVersions)
	stage2SNVersions := formatMultipleVersions(test.Stage2.StoragenodeVersions)
	stage1UplinkVersions := formatMultipleVersions(test.Stage1.UplinkVersions)
	stage2UplinkVersions := formatMultipleVersions(test.Stage2.UplinkVersions)
	cmd := exec.Command(scriptFile, test.Stage1.SatelliteVersion, stage1UplinkVersions, stage1SNVersions, test.Stage2.SatelliteVersion, stage2UplinkVersions, stage2SNVersions)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func formatMultipleVersions(snvs []string) string {
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

func getVersions(test *VersionsTest, filteredTagList []string) ([]string, error) {
	if len(test.Stage1.UplinkVersions) > 0 && len(test.Stage2.UplinkVersions) > 0 || len(filteredTagList) > 0 {
		return filteredTagList, nil
	}
	tags, err := exec.Command("bash", "-c", `git fetch --tags -q && git tag | sort | uniq | grep "v[0-9]"`).Output()
	if err != nil {
		return nil, err
	}
	stringTags := string(tags)
	tagList := strings.Split(strings.TrimSpace(stringTags), "\n")
	// skip specified versions if there's any
	for _, tag := range tagList {
		shouldSkip := false
		for _, skip := range SkippedVersions {
			if skip == tag {
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}
		filteredTagList = append(filteredTagList, tag)
	}
	return filteredTagList, nil
}
