# Generated Console REST API

The API defined in this package allows users to make requests related to their accounts, projects, and buckets, which would normally be performed within the Satellite UI.

This API is _not_ enabled in all production environments, and it is _not_ generally available to Storj customers.

## Available Endpoints

Generated detailed documentation for each endpoint implemented can be found [here](../apidocs.gen.md).

## Usage

Requires setting 'Authorization' header for requests. Users cannot currently generate their own REST API keys.

Example of request
```bash
curl  -i -L \
    -H "Accept: application/json" \
    -H 'Authorization: Bearer <key>' \
    -X GET \
    "https://satellite.qa.storj.io/api/v0/projects/"
```

## Successful responses
All the requests (except DELETE) have a non-empty response body for the resource that you're interacting with.

Example:

```json
{
  "project": {
    "name": "My Awesome Project",
    "description": "it is perfect"
  }
}
```

## Error responses
When an API endpoint returns an error (status code 4XX) it contains a JSON error response with 1 error field:

Example:

```json
{
  "error": "authorization key format is incorrect. Should be 'Bearer <key>'"
}
```

