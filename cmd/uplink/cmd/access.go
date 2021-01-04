// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

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
	Format      string `help:"format of credentials, use 'env' or 'aws' for using in scripts" default:""`
	AWSProfile  string `help:"if using --format=aws, output the --profile tag using this profile" default:""`
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
		RunE:  accessRegister,
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
	access, err := getAccessFromArgZeroOrConfig(inspectCfg, args)
	if err != nil {
		return errs.New("no access specified: %w", err)
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

func accessRegister(cmd *cobra.Command, args []string) (err error) {
	access, err := getAccessFromArgZeroOrConfig(inspectCfg, args)
	if err != nil {
		return errs.New("no access specified: %w", err)
	}

	accessKey, secretKey, endpoint, err := RegisterAccess(access, registerCfg.AuthService, registerCfg.Public)
	if err != nil {
		return err
	}
	switch registerCfg.Format {
	case "env": // export / set compatible format
		fmt.Printf("AWS_ACCESS_KEY_ID=%s\n", accessKey)
		fmt.Printf("AWS_SECRET_ACCESS_KEY=%s\n", secretKey)
		// note that AWS_ENDPOINT configuration is not natively utilized by the AWS CLI
		fmt.Printf("AWS_ENDPOINT=%s\n", endpoint)
	case "aws": // aws configuration commands
		profile := ""
		if registerCfg.AWSProfile != "" {
			profile = " --profile " + registerCfg.AWSProfile
			fmt.Printf("aws configure %s\n", profile)
		}
		fmt.Printf("aws configure %s set aws_access_key_id %s\n", profile, accessKey)
		fmt.Printf("aws configure %s set aws_secret_access_key %s\n", profile, secretKey)
		// note that this configuration is not natively utilized by the AWS CLI
		fmt.Printf("aws configure %s set s3.endpoint_url %s\n", profile, endpoint)
	default: // plain text
		fmt.Println("========== CREDENTIALS ===================================================================")
		fmt.Println("Access Key ID: ", accessKey)
		fmt.Println("Secret Key   : ", secretKey)
		fmt.Println("Endpoint     : ", endpoint)
	}
	return nil
}

func getAccessFromArgZeroOrConfig(config AccessConfig, args []string) (access *uplink.Access, err error) {
	if len(args) != 0 {
		access, err = inspectCfg.GetNamedAccess(args[0])
		if err != nil {
			return nil, err
		}
		if access != nil {
			return access, nil
		}
		return uplink.ParseAccess(args[0])
	}
	return inspectCfg.GetAccess()
}

// RegisterAccess registers an access grant with a Gateway Authorization Service.
func RegisterAccess(access *uplink.Access, authService string, public bool) (accessKey, secretKey, endpoint string, err error) {
	if authService == "" {
		return "", "", "", errs.New("no auth service address provided")
	}
	accesssSerialized, err := access.Serialize()
	if err != nil {
		return "", "", "", errs.Wrap(err)
	}
	postData, err := json.Marshal(map[string]interface{}{
		"access_grant": accesssSerialized,
		"public":       public,
	})
	if err != nil {
		return accessKey, "", "", errs.Wrap(err)
	}

	resp, err := http.Post(fmt.Sprintf("%s/v1/access", authService), "application/json", bytes.NewReader(postData))
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
