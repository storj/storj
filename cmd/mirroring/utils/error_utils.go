package utils

import (
	"errors"
	"strings"
)

//CombineErrors will combine all error messages from error array in a single error
func CombineErrors(erray []error) (err error) {
	if erray == nil || len(erray) == 0 {
		return nil
	}

	length := len(erray)
	var errorStrings []string

	for i := 0; i < length; i++ {
		if erray[i] != nil {
			errorStrings = append(errorStrings, /*getErrorSource(i) + */ erray[i].Error())
		}
	}

	if len(errorStrings) > 0 {
		err = errors.New(strings.Join(errorStrings, "\n"))
	}

	return
}

//NewError  - extends error with additional message
//Params:
//err       - error to extend
//message   - message to extend error
//Returns   - new error
func NewError(err error, message string) error {

	if err == nil {
		return nil
	}

	return errors.New(message + err.Error())
}

func getErrorSource(iteration int) string {

	switch iteration {
		case 0:
			return "Error from prime server "
		case 1:
			return "Error from alter server "
		default:
			return ""
	}
}
