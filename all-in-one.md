Steps to setup a local network
==============================

Run `make all-in-one` from the repo root. This will build new images and stand
them up locally. If you want to test a different version, run
`VERSION=latest make all-in-one`. If you want to stand them up by hand, run the
following:

1. `make images`
   If you don't want to build the images locally, you can skip this step.
   Otherwise, you'll need the version number presented here if you don't use
   latest in the next step:
   `Built version: c6cd912-all-in-one-go1.10`

2. `export VERSION=latest`
   Export the version of the network you want to run. Latest should be ok, but
   if you're testing something else, set the version here. ex: `c6cd912-all-in-one-go1.10`
   Usable images should be pushed to the Docker Hub:
   - https://hub.docker.com/r/storjlabs/storage-node/tags/
   - https://hub.docker.com/r/storjlabs/satellite/tags/
   - https://hub.docker.com/r/storjlabs/uplink/tags/

3. `docker-compose up -d storage-node`
   Create the first storage node.

4. `scripts/fix-mock-overlay`
   Fix the mock-overlay flag for the satellite. This is needed until the overlay
   network is populated from kademlia correctly.

5. `docker-compose up satellite uplink`
   Bring up the satellite and uplink

6. Visit http://localhost:7777 or use the aws tool with `--endpoint=http://localhost:7777`
