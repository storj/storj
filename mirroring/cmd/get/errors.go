package get

import "fmt"

type ArgsError struct {
	args []string
	argsLen int
}

func NewArgsError(args []string) *ArgsError {
	var error = new(ArgsError)
	error.args = args
	error.argsLen = len(args)

	return error
}

func(e *ArgsError) Error() string {
	var mainErrorString = "Invalid args, expected at least %d, provided %d"

	if e.argsLen < minArg {
		return fmt.Sprintf(mainErrorString, minArg, e.argsLen)
	}

	if e.argsLen > maxArg{
		return fmt.Sprintf(
			mainErrorString + ", where bucketname = %s, object = %s. Please provide exactly %d arguments",
			minArg,
			e.argsLen,
			e.args[0],
			e.args[1],
			maxArg)
	}

	return ""
}