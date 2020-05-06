// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package wizard

import (
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"golang.org/x/crypto/ssh/terminal"

	"storj.io/common/storj"
)

// PromptForAccessName handles user input for access name to be used with wizards
func PromptForAccessName() (string, error) {
	_, err := fmt.Printf("Choose an access name [\"default\"]: ")
	if err != nil {
		return "", err
	}

	var accessName string
	n, err := fmt.Scanln(&accessName)
	if err != nil && n != 0 {
		return "", err
	}

	if accessName == "" {
		return "default", nil
	}
	return accessName, nil
}

// PromptForSatellite handles user input for a satellite address to be used with wizards
func PromptForSatellite(cmd *cobra.Command) (string, error) {
	satellites := []string{
		"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777",
		"12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777",
		"121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@asia-east-1.tardigrade.io:7777",
	}

	_, err := fmt.Print("Select your satellite:\n")
	if err != nil {
		return "", err
	}

	for iterator, value := range satellites {
		nodeURL, err := storj.ParseNodeURL(value)
		if err != nil {
			return "", err
		}

		host, _, err := net.SplitHostPort(nodeURL.Address)
		if err != nil {
			return "", err
		}

		_, err = fmt.Printf("\t[%d] %s\n", iterator+1, host)
		if err != nil {
			return "", nil
		}
	}

	_, err = fmt.Print(`Enter number or satellite address as "<nodeid>@<address>:<port>" [1]: `)
	if err != nil {
		return "", err
	}

	var satelliteAddress string
	n, err := fmt.Scanln(&satelliteAddress)
	if err != nil {
		if n != 0 {
			return "", err
		}
		// fmt.Scanln cannot handle empty input
		satelliteAddress = satellites[0]
	}

	if len(satelliteAddress) == 0 {
		return "", errs.New("satellite address cannot be empty")
	}

	if len(satelliteAddress) == 1 {
		switch satelliteAddress {
		case "1":
			satelliteAddress = satellites[0]
		case "2":
			satelliteAddress = satellites[1]
		case "3":
			satelliteAddress = satellites[2]
		default:
			return "", errs.New("satellite address cannot be one character")
		}
	}

	nodeURL, err := storj.ParseNodeURL(satelliteAddress)
	if err != nil {
		return "", err
	}

	if nodeURL.ID.IsZero() {
		return "", errs.New(`missing node id, satellite address must be in the format "<nodeid>@<address>:<port>"`)
	}

	return satelliteAddress, nil
}

// PromptForAPIKey handles user input for an API key to be used with wizards
func PromptForAPIKey() (string, error) {
	_, err := fmt.Print("Enter your API key: ")
	if err != nil {
		return "", err
	}
	var apiKey string
	n, err := fmt.Scanln(&apiKey)
	if err != nil && n != 0 {
		return "", err
	}

	if apiKey == "" {
		return "", errs.New("API key cannot be empty")
	}

	return apiKey, nil
}

// PromptForEncryptionPassphrase handles user input for an encryption passphrase to be used with wizards
func PromptForEncryptionPassphrase() (string, error) {
	_, err := fmt.Print("Enter your encryption passphrase: ")
	if err != nil {
		return "", err
	}
	encKey, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	_, err = fmt.Println()
	if err != nil {
		return "", err
	}

	if len(encKey) == 0 {
		return "", errs.New("Encryption passphrase cannot be empty")
	}

	_, err = fmt.Print("Enter your encryption passphrase again: ")
	if err != nil {
		return "", err
	}
	repeatedEncKey, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	_, err = fmt.Println()
	if err != nil {
		return "", err
	}

	if !bytes.Equal(encKey, repeatedEncKey) {
		return "", errs.New("encryption passphrase does not match")
	}

	return string(encKey), nil
}

// PromptForTracing handles user input for consent to turn on tracing to be used with wizards.
func PromptForTracing() (bool, error) {
	_, err := fmt.Printf(`
With your permission, Tardigrade can automatically collect analytics information from your uplink CLI and send it to Storj Labs (makers of Tardigrade) to help improve the quality and performance of our products. This information is sent only with your consent and is submitted anonymously to Storj Labs: (y/n)
`)
	if err != nil {
		return false, err
	}

	var userConsent string
	n, err := fmt.Scanln(&userConsent)
	if err != nil {
		if n != 0 {
			return false, err
		}
		// fmt.Scanln cannot handle empty input
		userConsent = "n"
	}

	switch userConsent {
	case "y", "yes", "Y", "Yes":
		return true, nil
	default:
		return false, nil
	}

}
