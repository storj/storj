Steps to setup a local network
==============================

1. `export VERSION=latest`
   Export the version of the network you want to run. Latest should be ok, but
   if you're testing something else, set the version here.

2. `docker-compose up -d storage-node`
   Create the first storage node.

3. `./fix-mock-overlay`
   Fix the mock-overlay flag for the satellite. This is needed until the overlay
   network is populated from kademlia correctly.

4. `docker-compose up`
   Bring up the satellite and uplink

5. Visit http://localhost:7777 or use the aws tool with `--endpoint localhost:7777`
