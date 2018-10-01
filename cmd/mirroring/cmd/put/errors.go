package put

import (
	"fmt"
)

type InvalidArgsLenError struct {
	argsLen int
}

func NewInvalidArgsError(argsLen int) *InvalidArgsLenError {
	return &InvalidArgsLenError{argsLen}
}

func (e *InvalidArgsLenError) Error() string {
	return fmt.Sprintf("invalid args len, provided %d, should be 2", e.argsLen)
}

type invalidContextValueType struct {
	method string
}

func (e *invalidContextValueType) Error() string {
	return fmt.Sprintf("%s: invalid contet value type", e.method)
}

var uploadFolderInvalidContextType = &invalidContextValueType{"UploadFolder"}
var uploadFileInvalidContextType = &invalidContextValueType{"UploadFile"}