Purpose of this is to process an API Key to see if it matches the correct client.

**Import the following libraries with:**
`go get -u github.com/spf13/viper`
`go get -u github.com/spf13/pflag`

**To use, run in** *examples/auth/main.go*:
`$ go run main.go --key=yourkey`

Default api key is preset with the mocked headers. This will be changed later.

**Where this is going**:
We're going to be using macaroons to validate a token and permissions. This is a small step to building in that direction.