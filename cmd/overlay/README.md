TODO

# Overlay Network 

Documentation for developing and building the overlay network component of the Storj node. 

## Running as a cache server 

To run a cache, you'll need a running instance of Redis. 

Using docker is the fastest way to get a redis instance up and running. 

`docker run -p 6379:6379 --name -d redis`

Once you have that running, build the binary.

`go build cmd/overlay/main.go`

Then you can run the node with the -cache flag 

`./main -cache localhost:6379`

