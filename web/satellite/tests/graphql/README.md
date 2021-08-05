# GraphQL/Satellite Security Tests

![alt text](https://github.com/storj/storj/raw/main/resources/logo.png)

Storj is building a decentralized cloud storage network.

----

## Prerequisites

- Checkout and Compile storj-sim
- Makes sure that *#SATELLITE_0_ADDR* is configured for an ip for the satellite.

>Use an actual address and not the loopback adapter or localhost. The reason for this is that the container will need to connect to the host machine on port 10002 tcp.

`export SATELLITE_0_ADDR=172.20.47.132:10002`

- Start storj-sim by setting the --host switch

```storj-sim network --host 172.20.47.132 setup```

## Usage

*Commands must be executed from the root of the repository*

### Introspection and API Test

```bash
web/satellite/tests/graphql/test_graphql.sh
```

### Introspection only

```bash
go run web/satellite/tests/graphql/main.go
```

### API Tests only

```bash
docker pull postman/newman:alpine
docker run --network="host" -v ${PWD}/web/satellite/tests/graphql/:/etc/newman -t postman/newman:alpine run GraphQL.postman_collection.json -e GraphQLEndoints.postman_environment.json
```

## License

The network under construction (this repo) is currently licensed with the
[AGPLv3](https://www.gnu.org/licenses/agpl-3.0.en.html) license. Once the network
reaches beta phase, we will be licensing all client-side code via the
[Apache v2](https://www.apache.org/licenses/LICENSE-2.0) license.

For code released under the AGPLv3, we request that contributors sign our
[Contributor License Agreement (CLA)](https://docs.google.com/forms/d/e/1FAIpQLSdVzD5W8rx-J_jLaPuG31nbOzS8yhNIIu4yHvzonji6NeZ4ig/viewform) so that we can relicense the
code under Apache v2, or other licenses in the future.
