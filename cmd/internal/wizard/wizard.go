// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package wizard

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"golang.org/x/term"

	"storj.io/common/storj"
)

// PromptForAccessName handles user input for access name to be used with wizards.
func PromptForAccessName() (string, error) {
	_, err := fmt.Printf("Choose an access name (use lowercase letters) [\"default\"]: ")
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

	if accessName != strings.ToLower(accessName) {
		return "", errs.New("Please only use lowercase letters for access name.")
	}

	return accessName, nil
}

// PromptForSatellite handles user input for a satellite address to be used with wizards.
func PromptForSatellite(cmd *cobra.Command) (string, error) {
	satellites := []string{
		"12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us1.storj.io:7777",
		"12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@eu1.storj.io:7777",
		"121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@ap1.storj.io:7777",
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
			return "", nil //nolint: nilerr // we'll skip the prompt, when there's an error
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
		satIdx, err := strconv.Atoi(satelliteAddress)
		if err != nil {
			return "", errs.New("invalid satellite address option")
		}

		if satIdx < 1 || satIdx > len(satellites) {
			return "", errs.New("invalid satellite address option")
		}

		satelliteAddress = satellites[satIdx-1]
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

// PromptForAPIKey handles user input for an API key to be used with wizards.
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

// PromptForEncryptionPassphrase handles user input for an encryption passphrase to be used with wizards.
func PromptForEncryptionPassphrase() (string, error) {
	_, err := fmt.Print(`Data is encrypted on the network, with an encryption passphrase
stored on your local machine. Enter a passphrase you'd like to use.
Enter your encryption passphrase: `)
	if err != nil {
		return "", err
	}
	encKey, err := term.ReadPassword(int(os.Stdin.Fd()))
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
	repeatedEncKey, err := term.ReadPassword(int(os.Stdin.Fd()))
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
With your permission, Storj can automatically collect analytics information from your uplink CLI to help improve the quality and performance of our products. This information is sent only with your consent and is submitted anonymously to Storj Labs: (y/n)
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
