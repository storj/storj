// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package cfgstruct

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"golang.org/x/crypto/ssh/terminal"
)

// applyDefaultHostAndPortToAddr applies the default host and/or port if either is missing in the specified address.
func applyDefaultHostAndPortToAddr(address, defaultAddress string) (string, error) {
	defaultHost, defaultPort, err := net.SplitHostPort(defaultAddress)
	if err != nil {
		return "", err
	}

	addressParts := strings.Split(address, ":")
	numberOfParts := len(addressParts)

	if numberOfParts > 1 && len(addressParts[0]) > 0 && len(addressParts[1]) > 0 {
		// address is host:port so skip applying any defaults.
		return address, nil
	}

	// We are missing a host:port part. Figure out which part we are missing.
	indexOfPortSeparator := strings.Index(address, ":")
	lengthOfFirstPart := len(addressParts[0])

	if indexOfPortSeparator < 0 {
		if lengthOfFirstPart == 0 {
			// address is blank.
			return defaultAddress, nil
		}
		// address is host
		return net.JoinHostPort(addressParts[0], defaultPort), nil
	}

	if indexOfPortSeparator == 0 {
		// address is :1234
		return net.JoinHostPort(defaultHost, addressParts[1]), nil
	}

	// address is host:
	return net.JoinHostPort(addressParts[0], defaultPort), nil
}

// PromptForSatelitte handles user input for a satelitte address to be used with wizards
func PromptForSatelitte(cmd *cobra.Command) (string, error) {
	_, err := fmt.Print(`
	Pick satellite to use:
	[1] mars.tardigrade.io
	[2] jupiter.tardigrade.io
	[3] saturn.tardigrade.io
	Please enter numeric choice or enter satellite address manually [1]: `)
	if err != nil {
		return "", err
	}
	satellites := []string{"mars.tardigrade.io", "jupiter.tardigrade.io", "saturn.tardigrade.io"}
	var satelliteAddress string
	n, err := fmt.Scanln(&satelliteAddress)
	if err != nil {
		if n == 0 {
			// fmt.Scanln cannot handle empty input
			satelliteAddress = satellites[0]
		} else {
			return "", err
		}
	}

	// TODO add better validation
	if satelliteAddress == "" {
		return "", errs.New("satellite address cannot be empty")
	} else if len(satelliteAddress) == 1 {
		switch satelliteAddress {
		case "1":
			satelliteAddress = satellites[0]
		case "2":
			satelliteAddress = satellites[1]
		case "3":
			satelliteAddress = satellites[2]
		default:
			return "", errs.New("Satellite address cannot be one character")
		}
	}

	return applyDefaultHostAndPortToAddr(satelliteAddress, cmd.Flags().Lookup("satellite-addr").Value.String())
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

// PromptForEncryptionKey handles user input for an encryption key to be used with wizards
func PromptForEncryptionKey() (string, error) {
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
		return "", errs.New("encryption passphrases doesn't match")
	}

	if len(encKey) == 0 {
		_, err = fmt.Println("Warning: Encryption passphrase is empty!")
		if err != nil {
			return "", err
		}
	}

	return string(encKey), nil
}
