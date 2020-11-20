// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcutil/base58"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/uplink"
)

type registerConfig struct {
	AuthService string `help:"the address to the service you wish to register your access with" default:"" basic-help:"true"`
	Public      bool   `help:"if the access should be public" default:"false" basic-help:"true"`
	AWSProfile  string `help:"update AWS credentials file, appending the credentials using this profile name" default:"" basic-help:"true"`
	AccessConfig
}

var (
	inspectCfg  AccessConfig
	listCfg     AccessConfig
	registerCfg registerConfig
	awsErr      = errs.Class("error writing AWS credentials file")
)

func init() {
	// We skip the use of addCmd here because we only want the configuration options listed
	// above, and addCmd adds a whole lot more than we want.
	accessCmd := &cobra.Command{
		Use:   "access",
		Short: "Set of commands to manage access.",
	}

	inspectCmd := &cobra.Command{
		Use:   "inspect [ACCESS]",
		Short: "Inspect allows you to explode a serialized access into it's constituent parts.",
		RunE:  accessInspect,
		Args:  cobra.MaximumNArgs(1),
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Prints name and associated satellite of all available accesses.",
		RunE:  accessList,
		Args:  cobra.NoArgs,
	}

	registerCmd := &cobra.Command{
		Use:   "register [ACCESS]",
		Short: "Register your access for use with a hosted gateway.",
		RunE:  registerAccess,
		Args:  cobra.MaximumNArgs(1),
	}

	RootCmd.AddCommand(accessCmd)
	accessCmd.AddCommand(inspectCmd)
	accessCmd.AddCommand(listCmd)
	accessCmd.AddCommand(registerCmd)

	process.Bind(inspectCmd, &inspectCfg, defaults, cfgstruct.ConfDir(getConfDir()))
	process.Bind(listCmd, &listCfg, defaults, cfgstruct.ConfDir(getConfDir()))
	process.Bind(registerCmd, &registerCfg, defaults, cfgstruct.ConfDir(getConfDir()))
}

func accessList(cmd *cobra.Command, args []string) (err error) {
	accesses := listCfg.Accesses
	fmt.Println("=========== ACCESSES LIST: name / satellite ================================")
	for name, data := range accesses {
		satelliteAddr, _, _, err := parseAccess(data)
		if err != nil {
			return err
		}

		fmt.Println(name, "/", satelliteAddr)
	}
	return nil
}

func accessInspect(cmd *cobra.Command, args []string) (err error) {
	var access *uplink.Access
	if len(args) == 0 {
		access, err = inspectCfg.GetAccess()
		if err != nil {
			return err
		}
	} else {
		firstArg := args[0]

		access, err = inspectCfg.GetNamedAccess(firstArg)
		if err != nil {
			return err
		}

		if access == nil {
			if access, err = uplink.ParseAccess(firstArg); err != nil {
				return err
			}
		}
	}

	serializedAccesss, err := access.Serialize()
	if err != nil {
		return err
	}

	satAddr, apiKey, ea, err := parseAccess(serializedAccesss)
	if err != nil {
		return err
	}

	fmt.Println("=========== ACCESS INFO ==================================================================")
	fmt.Println("Satellite        :", satAddr)
	fmt.Println("API Key          :", apiKey)
	fmt.Println("Encryption Access:", ea)
	return nil
}

func parseAccess(access string) (sa string, apiKey string, ea string, err error) {
	data, version, err := base58.CheckDecode(access)
	if err != nil || version != 0 {
		return "", "", "", errs.New("invalid access grant format: %v", err)
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return "", "", "", err
	}

	eaData, err := pb.Marshal(p.EncryptionAccess)
	if err != nil {
		return "", "", "", errs.New("unable to marshal encryption access: %v", err)
	}

	apiKey = base58.CheckEncode(p.ApiKey, 0)
	ea = base58.CheckEncode(eaData, 0)
	return p.SatelliteAddr, apiKey, ea, nil
}

func registerAccess(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return errs.New("no access specified")
	}

	if registerCfg.AuthService == "" {
		return errs.New("no auth service address provided")
	}

	accessRaw := args[0]

	// try assuming that accessRaw is a named access
	access, err := registerCfg.GetNamedAccess(accessRaw)
	if err == nil && access != nil {
		accessRaw, err = access.Serialize()
		if err != nil {
			return errs.New("error serializing named access '%s': %v", accessRaw, err)
		}
	}

	postData, err := json.Marshal(map[string]interface{}{
		"access_grant": accessRaw,
		"public":       registerCfg.Public,
	})
	if err != nil {
		return errs.Wrap(err)
	}

	resp, err := http.Post(fmt.Sprintf("%s/v1/access", registerCfg.AuthService), "application/json", bytes.NewReader(postData))
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	respBody := make(map[string]string)
	if err := json.Unmarshal(body, &respBody); err != nil {
		return errs.New("unexpected response from auth service: %s", string(body))
	}

	accessKey, ok := respBody["access_key_id"]
	if !ok {
		return errs.New("access_key_id missing in response")
	}
	secretKey, ok := respBody["secret_key"]
	if !ok {
		return errs.New("secret_key missing in response")
	}
	fmt.Println("=========== CREDENTIALS =========================================================")
	fmt.Println("Access Key ID: ", accessKey)
	fmt.Println("Secret Key:    ", secretKey)
	fmt.Println("Endpoint:      ", respBody["endpoint"])

	return tryWriteAWSCredentials(registerCfg.AWSProfile, accessKey, secretKey)
}

// tryWriteAWSCredentials appends to ~/.aws/credentials or %USERPROFILE%\.aws\credentials.
func tryWriteAWSCredentials(profileName, accessKey, secretKey string) error {
	if registerCfg.AWSProfile == "" {
		return nil
	}
	credPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return awsErr.New("cannot determine home directory: %v", err)
		}
		credPath = filepath.Join(homeDir, ".aws", "credentials")
	}
	// open file
	f, err := os.OpenFile(credPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return awsErr.New("error opening AWS credentials file: %v", err)
	}
	defer func() { _ = f.Close() }()
	// write data
	if _, err := fmt.Fprintf(f, "\n[%s]\n", profileName); err != nil {
		return awsErr.New("error writing to AWS credentials file: %v", err)
	}
	if _, err := fmt.Fprintf(f, "aws_access_key_id = %s\n", accessKey); err != nil {
		return awsErr.New("error writing to AWS credentials file: %v", err)
	}
	if _, err := fmt.Fprintf(f, "aws_secret_access_key = %s\n", secretKey); err != nil {
		return awsErr.New("error writing to AWS credentials file: %v", err)
	}
	fmt.Printf("Updated AWS credential file %s with profile '%s'\n", credPath, profileName)
	return nil
}
