#!/bin/bash
set -ueo pipefail

# Purpose: This script executes uplink upload and download benchmark tests against storj-sim.
# Setup: Setup and run storj-sim. Remove existing uplink configs.
# Usage: from the root of storj dir, run ./scripts/test-sim-benchmark.sh

# create an api key that the uplink will use
regEndpoint=http://127.0.0.1:10002/registrationToken/?projectsLimit=1;
endpoint=http://127.0.0.1:10002/api/graphql/v0;
satellitePublicGRPC=http://127.0.0.1:10000;

email=`date +%s`@email.com;
projectName=project`date +%s`;

# In order to create an api key, do the following 5 steps:
# 1. create reg token
secret=$(curl -X GET -H "Authorization: secure_token" $regEndpoint 2>/dev/null | awk -F'"' '$2=="secret"{print $4}');

# 2. create user
echo "Created new user:"
curl -X POST -H "Content-Type: application/graphql" -d "mutation {createUser(input:{email:\"$email\",password:\"123a123\",fullName:\"McTesterson\", shortName:\"\"}, secret:\"$secret\" ){id,email,createdAt}}" $endpoint;

# 3. make service token request
token=$(curl -X POST -H "Content-Type: application/graphql" -d "query {token(email:\"$email\",password:\"123a123\"){token,user{shortName,id}}}" $endpoint 2>/dev/null | awk -F'"' '$2=="data"{print $8}');

# 4. create project
projectID=$(curl -X POST -H "Authorization: Bearer $token" -H "Content-Type: application/graphql" -d "mutation {createProject(input:{name:\"$projectName\",description:\"things\"}){name,description,id,createdAt}}" $endpoint 2>/dev/null | awk -F'"' '$2=="data"{print $16}');
echo
echo "Created new project:"
echo $projectID

# 5. create api key
apiKey=$(curl -X POST -H "Authorization: Bearer $token" -H "Content-Type: application/graphql" -d "mutation {createAPIKey(projectID:\"$projectID\",name:\"$projectName\"){keyInfo{name,id,createdAt,projectID},key}}" $endpoint 2>/dev/null | awk -F'"' '$2=="data"{print $8}');
echo
echo "Created new api key:"
echo $apiKey

# export api key as an environment variable for the benchmark tests to use
eval 'export storjSimApiKey=$apiKey'
echo
echo "storjSimApiKey:"
echo $storjSimApiKey

# run benchmark tests normally
echo
echo "Executing benchmark tests locally with no latency..."
go test -bench . -benchmem ./cmd/uplink/cmd/

# TODO(jg): run benchmark tests with latency

# run s3-benchmark with uplink
s3-benchmark --client=uplink --apikey=$storjSimApiKey --satellite=$satellitePublicGRPC
