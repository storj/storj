# satellite/admin

Satellite Admin package provides API endpoints for administrative tasks.

Requires setting `Authorization` header for requests.

<!-- Auto-generate this ToC with https://github.com/ycd/toc -->
<!-- toc -->
- [satellite/admin](#satelliteadmin)
    * [API design](#api-design)
        * [Successful responses](#successful-responses)
        * [Error responses](#error-responses)
    * [API Endpoints](#api-endpoints)
        * [User Management](#user-management)
            * [POST /api/users](#post-apiusers)
            * [PUT /api/users/{user-email}](#put-apiusersuser-email)
            * [GET /api/users/{user-email}](#get-apiusersuser-email)
            * [GET /api/users/{user-email}/limits](#get-apiusersuser-emaillimits)
            * [DELETE /api/users/{user-email}](#delete-apiusersuser-email)
            * [PUT /api/users/{user-email}/limits](#put-apiusersuser-emaillimits)
            * [DELETE /api/users/{user-email}/mfa](#delete-apiusersuser-emailmfa)
            * [PUT /api/users/{user-email}/freeze](#put-apiusersuser-emailfreeze)
            * [DELETE /api/users/{user-email}/freeze](#delete-apiusersuser-emailfreeze)
        * [OAuth Client Management](#oauth-client-management)
            * [POST /api/oauth/clients](#post-apioauthclients)
            * [PUT /api/oauth/clients/{id}](#put-apioauthclientsid)
            * [DELETE /api/oauth/clients/{id}](#delete-apioauthclientsid)
        * [Project Management](#project-management)
            * [POST /api/projects](#post-apiprojects)
            * [GET /api/projects/{project-id}](#get-apiprojectsproject-id)
            * [PUT /api/projects/{project-id}](#put-apiprojectsproject-id)
            * [DELETE /api/projects/{project-id}](#delete-apiprojectsproject-id)
            * [GET /api/projects/{project}/apikeys](#get-apiprojectsprojectapikeys)
            * [POST /api/projects/{project}/apikeys](#post-apiprojectsprojectapikeys)
            * [DELETE /api/projects/{project}/apikeys/{name}](#delete-apiprojectsprojectapikeysname)
            * [GET /api/projects/{project-id}/usage](#get-apiprojectsproject-idusage)
            * [GET /api/projects/{project-id}/limit](#get-apiprojectsproject-idlimit)
            * [Update limits](#update-limits)
                * [POST /api/projects/{project-id}/limit?usage={value}](#post-apiprojectsproject-idlimitusagevalue)
                * [POST /api/projects/{project-id}/limit?bandwidth={value}](#post-apiprojectsproject-idlimitbandwidthvalue)
                * [POST /api/projects/{project-id}/limit?rate={value}](#post-apiprojectsproject-idlimitratevalue)
                * [POST /api/projects/{project-id}/limit?buckets={value}](#post-apiprojectsproject-idlimitbucketsvalue)
                * [POST /api/projects/{project-id}/limit?burst={value}](#post-apiprojectsproject-idlimitburstvalue)
                * [POST /api/projects/{project-id}/limit?segments={value}](#post-apiprojectsproject-idlimitsegmentsvalue)
        * [Bucket Management](#bucket-management)
            * [GET /api/projects/{project-id}/buckets/{bucket-name}](#get-apiprojectsproject-idbucketsbucket-name)
            * [Geofencing](#geofencing)
                * [POST /api/projects/{project-id}/buckets/{bucket-name}/geofence?region={value}](#post-apiprojectsproject-idbucketsbucket-namegeofenceregionvalue)
                * [DELETE /api/projects/{project-id}/buckets/{bucket-name}/geofence](#delete-apiprojectsproject-idbucketsbucket-namegeofence)
        * [APIKey Management](#apikey-management)
            * [GET /api/apikeys/{apikey}](#get-apiapikeysapikey)
            * [DELETE /api/apikeys/{apikey}](#delete-apiapikeysapikey)

<!-- tocstop -->

## API design

### Successful responses

For non-get requests (`PUT`, `POST`, `DELETE`), endpoints should return an empty response body on success. `GET`
requests can return a non-empty body for the resource that we're interacting with.

### Error responses

When an API endpoint returns a client error (status code 4XX) it returns a JSON error response which contains 2 fields:

* `error`: The error message.
* `detail` (may be empty): Some detail about the returned error.

Example:

```json
{
  "error": "usage for the current month exists",
  "detail": ""
}
```

## API Endpoints
### User Management

#### POST /api/users

Adds a new user.

An example of a required request body:

```json
{
    "email": "alice@mail.test",
    "fullName": "Alice Test",
    "password": "password"
}
```

A successful response body:

```json
{
    "id":           "12345678-1234-1234-1234-123456789abc",
    "email":        "alice@mail.test",
    "fullName":     "Alice Test",
    "shortName":    "",
    "passwordHash": ""
}
```

#### PUT /api/users/{user-email}

Updates the details of existing user found by its email.

Some example request bodies:

```json
{
    "email": "alice+2@mail.test"
}
```

```json
{
    "email": "alice+2@mail.test",
    "shortName": "myNickName"
}
```

```json
{
    "projectLimit": 200
}
```

#### GET /api/users/{user-email}

This endpoint returns information about user and their projects.

A successful response body:

```json
{
    "user":{
        "id": "12345678-1234-1234-1234-123456789abc",
        "fullName": "Alice Bob",
        "email":"alice@example.test",
        "projectLimit": 10
    },
    "projects":[
        {
            "id": "abcabcab-1234-abcd-abcd-abecdefedcab",
            "name": "Project",
            "description": "Project to store data.",
            "ownerId": "12345678-1234-1234-1234-123456789abc"
        }
    ]
}
```

#### GET /api/users/{user-email}/limits

This endpoint returns information about users limits.

#### DELETE /api/users/{user-email}

Deletes the user.

#### PUT /api/users/{user-email}/limits

Updates the limits of the user and user's existing project(s) limits found by its email.

#### DELETE /api/users/{user-email}/mfa

Disables the user's mfa.

#### PUT /api/users/{user-email}/freeze

Freezes a user account so no uploads or downloads may occur.

#### DELETE /api/users/{user-email}/freeze

Unfreezes a user account so uploads and downloads may resume.

### OAuth Client Management

Manages oauth clients known to the Satellite.

#### POST /api/oauth/clients

Create a new OAuthClient. A client ID and clientSecret will be returned upon creation.

Example request:

```json
{
  "id": "uuid-of-the-client",
  "secret": "shh-this-should-be-kept-safe",
  "redirectURL": "http://localhost:8888/oauth/storj/callback",
  "userID": "uuid-of-the-owner",
  "appName": "Example App",
  "appLogoURL": "http://localhost:8888/logo.png"
}
```

#### PUT /api/oauth/clients/{id}

Update an existing oauth client.

Example request:

```json
{
  "redirectURL": "http://localhost:8888/oauth/storj/callback",
  "appName": "Example App",
  "appLogoURL": "http://localhost:8888/logo.png"
}
```

#### DELETE /api/oauth/clients/{id}

Delete the identified oauth client.

### Project Management

#### POST /api/projects

Adds a project for specific user.

An example of a required request body:

```json
{
    "ownerId": "ca7aa0fb-442a-4d4e-aa36-a49abddae837",
    "projectName": "My Second Project"
}
```

A successful response body:

```json
{
    "projectId": "ca7aa0fb-442a-4d4e-aa36-a49abddae646"
}
```

#### GET /api/projects/{project-id}

Gets the common information about a project.

#### PUT /api/projects/{project-id}

Updates project name or description.

```json
{
    "projectName": "My new Project Name",
    "description": "My new awesome description!"
}
```

#### DELETE /api/projects/{project-id}

Deletes the project.

#### GET /api/projects/{project}/apikeys

Get the list of the API keys of a specific project.

A successful response body:

```json
[
    {
        "id": "b6988bd2-8d21-4bee-91ac-a3445bf38180",
        "ownerId": "ca7aa0fb-442a-4d4e-aa36-a49abddae837",
        "name": "mine",
        "partnerID": "a9d3b7ee-17da-4848-bb0e-1f64cf45af18",
        "createdAt": "2020-05-19T00:34:13.265761+02:00"
    },
    {
        "id": "f9f887c1-b178-4eb8-b669-14379c5a97ca",
        "ownerId": "3eb45ae9-822a-470e-a51a-9144dedda63e",
        "name": "family",
        "partnerID": "",
        "createdAt": "2020-02-20T15:34:24.265761+02:00"
    }
]
```

#### POST /api/projects/{project}/apikeys

Adds an apikey for specific project.

An example of a required request body:

```json
{
    "name": "My first API Key"
}
```
**Note:** Additionally you can specify `partnerId` to associate it with the given apikey.
If you specify it, it has to be a valid uuid and not an empty string.

A successful response body:

```json
{
    "apikey": "13YqdMKxAVBamFsS6Mj3sCQ35HySoA254xmXCCQGJqffLnqrBaQDoTcCiCfbkaFPNewHT79rrFC5XRm4Z2PENtRSBDVNz8zcjS28W5v"
}
```

#### DELETE /api/projects/{project}/apikeys/{name}

Deletes the given apikey by its name.

#### GET /api/projects/{project-id}/usage

This endpoint returns whether the project has outstanding usage or not.

A project with not usage returns status code 200 and `{"result":"no project usage exist"}`.
Otherwise, it returns status code 409 with a JSON error.`{"error":"usage for current month exists""}`.

#### GET /api/projects/{project-id}/limit

This endpoint returns information about project limits.

A successful response body:

```json
{
  "usage": {
    "amount": "1.0 TB",
    "bytes": 1000000000000
  },
  "bandwidth": {
    "amount": "1.0 TB",
    "bytes": 1000000000000
  },
  "rate": {
    "rps": 0
  },
  "maxBuckets": 1000,
  "maxSegments": 1000000000
}
```

#### Update limits

You can update the different limits with one single request just adding the
various query parameters (e.g. `usage=5000000&bandwidth=9000000`)

##### POST /api/projects/{project-id}/limit?usage={value}

Updates usage limit for a project. The value must be in bytes.

##### POST /api/projects/{project-id}/limit?bandwidth={value}

Updates bandwidth limit for a project. The value must be in bytes.

##### POST /api/projects/{project-id}/limit?rate={value}

Updates rate limit for a project.

##### POST /api/projects/{project-id}/limit?buckets={value}

Updates number of buckets limit for a project.

##### POST /api/projects/{project-id}/limit?burst={value}

Updates burst limit for a project.

##### POST /api/projects/{project-id}/limit?segments={value}

Updates number of segments limit for a project.

### Bucket Management

This set of APIs provide administrative functionality over bucket functionality.

#### GET /api/projects/{project-id}/buckets/{bucket-name}

Returns all the information of the specified bucket.

#### Geofencing

Manage geofencing capabilities for a given bucket.

##### POST /api/projects/{project-id}/buckets/{bucket-name}/geofence?region={value}

Enables the geofencing configuration for the specified bucket. The bucket MUST be empty in order for this to work. Valid
values for the `region` parameter are:

- `EU` - restrict placement to data nodes that reside in the [European Union][]
- `EEA` - restrict placement to data nodes that reside in the [European Economic Area][]
- `US` - restricts placement to data nodes in the United States
- `DE` - restricts placement to data nodes in Germany

[European Union]: https://github.com/storj/common/blob/main/storj/location/region.go#L14

[European Economic Area]: https://github.com/storj/common/blob/main/storj/location/region.go#L7

##### DELETE /api/projects/{project-id}/buckets/{bucket-name}/geofence

Removes the geofencing configuration for the specified bucket. The bucket MUST be empty in order for this to work.

### APIKey Management

#### GET /api/apikeys/{apikey}

Gets information on the given apikey.

A successful response body:

```json
{
  "api_key": {
    "id": "12345678-1234-1234-1234-123456789abc",
    "name": "my key",
    "createdAt": "2020-05-19T00:34:13.265761+02:00"
  },
  "project": {
    "id": "12345678-1234-1234-1234-123456789abc",
    "name": "My Project",
  },
  "owner": {
    "id": "12345678-1234-1234-1234-123456789abc",
    "fullName": "test user",
    "email": "bob@example.test",
    "paidTier": true
  }
}
```

#### DELETE /api/apikeys/{apikey}

Deletes the given apikey.
