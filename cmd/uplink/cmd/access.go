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

	"storj.io/common/fpath"
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
		return "", "", "", errs.New("invalid access grant format: %w", err)
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return "", "", "", err
	}

	eaData, err := pb.Marshal(p.EncryptionAccess)
	if err != nil {
		return "", "", "", errs.New("unable to marshal encryption access: %w", err)
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
			return errs.New("error serializing named access '%s': %w", accessRaw, err)
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

	// update AWS credential file if requested
	if registerCfg.AWSProfile != "" {
		credentialsPath, err := getAwsCredentialsPath()
		if err != nil {
			return err
		}
		err = writeAWSCredentials(credentialsPath, registerCfg.AWSProfile, accessKey, secretKey)
		if err != nil {
			return err
		}
	}
	return nil
}

// getAwsCredentialsPath returns the expected AWS credentials path.
func getAwsCredentialsPath() (string, error) {
	if credentialsPath, found := os.LookupEnv("AWS_SHARED_CREDENTIALS_FILE"); found {
		return credentialsPath, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errs.Wrap(err)
	}
	return filepath.Join(homeDir, ".aws", "credentials"), nil
}

// writeAWSCredentials appends to credentialsPath using an AWS compliant credential formatting.
func writeAWSCredentials(credentialsPath, profileName, accessKey, secretKey string) error {
	oldCredentials, err := ioutil.ReadFile(credentialsPath)
	if err != nil && !os.IsNotExist(err) {
		return errs.Wrap(err)
	}
	const format = "\n[%s]\naws_access_key_id = %s\naws_secret_access_key = %s\n"
	newCredentials := fmt.Sprintf(format, profileName, accessKey, secretKey)

	var fileMode os.FileMode
	fileInfo, err := os.Stat(credentialsPath)
	if err == nil {
		fileMode = fileInfo.Mode()
	} else {
		fileMode = 0644
	}
	err = fpath.AtomicWriteFile(credentialsPath, append(oldCredentials, newCredentials...), fileMode)
	if err != nil {
		return errs.Wrap(err)
	}
	fmt.Printf("Updated AWS credential file %s with profile '%s'\n", credentialsPath, profileName)
	return nil
}
