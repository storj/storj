package main

import "fmt"

func wrapf(message string, err error) error {
	if err != nil {
		return fmt.Errorf(message, err)
	}
	return nil
}
