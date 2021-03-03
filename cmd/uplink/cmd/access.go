// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/uplink"
)

const defaultAccessRegisterTimeout = 15 * time.Second

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

type base64url []byte

func (b base64url) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.URLEncoding.EncodeToString(b) + `"`), nil
}

type accessInfo struct {
	SatelliteAddr    string               `json:"satellite_addr"`
	EncryptionAccess *pb.EncryptionAccess `json:"encryption_access"`
	APIKey           string               `json:"api_key"`
	Macaroon         accessInfoMacaroon   `json:"macaroon"`
}

type accessInfoMacaroon struct {
	Head    base64url         `json:"head"`
	Caveats []macaroon.Caveat `json:"caveats"`
	Tail    base64url         `json:"tail"`
}

func accessInspect(cmd *cobra.Command, args []string) (err error) {
	// FIXME: This is inefficient. We end up parsing, serializing, parsing
	// again. It can get particularly bad with large access grants.
	access, err := getAccessFromArgZeroOrConfig(inspectCfg, args)
	if err != nil {
		return errs.New("no access specified: %w", err)
	}

	serializedAccesss, err := access.Serialize()
	if err != nil {
		return err
	}

	p, err := parseAccessRaw(serializedAccesss)
	if err != nil {
		return err
	}

	m, err := macaroon.ParseMacaroon(p.ApiKey)
	if err != nil {
		return err
	}

	// TODO: this could be better
	apiKey, err := macaroon.ParseRawAPIKey(p.ApiKey)
	if err != nil {
		return err
	}

	ai := accessInfo{
		SatelliteAddr:    p.SatelliteAddr,
		EncryptionAccess: p.EncryptionAccess,
		APIKey:           apiKey.Serialize(),
		Macaroon: accessInfoMacaroon{
			Head:    m.Head(),
			Caveats: []macaroon.Caveat{},
			Tail:    m.Tail(),
		},
	}

	for _, cb := range m.Caveats() {
		var c macaroon.Caveat

		err := pb.Unmarshal(cb, &c)
		if err != nil {
			return err
		}

		ai.Macaroon.Caveats = append(ai.Macaroon.Caveats, c)
	}

	bs, err := json.MarshalIndent(ai, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(bs))

	return nil
}

func parseAccessRaw(access string) (_ *pb.Scope, err error) {
	data, version, err := base58.CheckDecode(access)
	if err != nil || version != 0 {
		return nil, errs.New("invalid access grant format: %w", err)
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return nil, err
	}

	return p, nil
}

func parseAccess(access string) (sa string, apiKey string, ea string, err error) {
	p, err := parseAccessRaw(access)
	if err != nil {
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
	access, err := getAccessFromArgZeroOrConfig(registerCfg.AccessConfig, args)
	if err != nil {
		return errs.New("no access specified: %w", err)
	}

	accessKey, secretKey, endpoint, err := RegisterAccess(access, registerCfg.AuthService, registerCfg.Public, defaultAccessRegisterTimeout)
	if err != nil {
		return err
	}

	return DisplayGatewayCredentials(accessKey, secretKey, endpoint, registerCfg.Format, registerCfg.AWSProfile)
}

func getAccessFromArgZeroOrConfig(config AccessConfig, args []string) (access *uplink.Access, err error) {
	if len(args) != 0 {
		access, err = config.GetNamedAccess(args[0])
		if err != nil {
			return nil, err
		}
		if access != nil {
			return access, nil
		}
		return uplink.ParseAccess(args[0])
	}
	return config.GetAccess()
}

// DisplayGatewayCredentials formats and writes credentials to stdout.
func DisplayGatewayCredentials(accessKey, secretKey, endpoint, format, awsProfile string) (err error) {
	switch format {
	case "env": // export / set compatible format
		// note that AWS_ENDPOINT configuration is not natively utilized by the AWS CLI
		_, err = fmt.Printf("AWS_ACCESS_KEY_ID=%s\n"+
			"AWS_SECRET_ACCESS_KEY=%s\n"+
			"AWS_ENDPOINT=%s\n",
			accessKey, secretKey, endpoint)
		if err != nil {
			return err
		}
	case "aws": // aws configuration commands
		profile := ""
		if awsProfile != "" {
			profile = " --profile " + awsProfile
			_, err = fmt.Printf("aws configure %s\n", profile)
			if err != nil {
				return err
			}
		}
		// note that the endpoint_url configuration is not natively utilized by the AWS CLI
		_, err = fmt.Printf("aws configure %s set aws_access_key_id %s\n"+
			"aws configure %s set aws_secret_access_key %s\n"+
			"aws configure %s set s3.endpoint_url %s\n",
			profile, accessKey, profile, secretKey, profile, endpoint)
		if err != nil {
			return err
		}
	default: // plain text
		_, err = fmt.Printf("========== CREDENTIALS ===================================================================\n"+
			"Access Key ID: %s\n"+
			"Secret Key   : %s\n"+
			"Endpoint     : %s\n",
			accessKey, secretKey, endpoint)
		if err != nil {
			return err
		}
	}
	return nil
}

// RegisterAccess registers an access grant with a Gateway Authorization Service.
func RegisterAccess(access *uplink.Access, authService string, public bool, timeout time.Duration) (accessKey, secretKey, endpoint string, err error) {
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

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Post(fmt.Sprintf("%s/v1/access", authService), "application/json", bytes.NewReader(postData))
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
