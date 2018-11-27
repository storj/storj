# Inspector CLI

Inspector is a CLI tool for inspecting and interacting with different Storj services for development. It is not included in the Satellite production build.

## Using inspector

1. Start up CaptPlanet

2. Install Inspector with `go install ./cmd/inspector/` or just run straight from the main.go with `go run cmd/inspector/main.go`

By default, the inspector should point at the correct port and host for CaptPlanet (currently this is set at `[::1]:7778`, but if you need to change it, you can pass the `-address` flag with the correct address. 


## Service subcommands
Each service has a name-spaced command with associated subcommands. For example, if you want to use a Kad command, it would look like

`inspector kad list-buckets`

## Help
`inspector --help` will dump out a help menu for more documentation. 
