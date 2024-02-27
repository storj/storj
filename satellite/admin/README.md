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
            * [PUT /api/users/{user-email}/billing-freeze](#put-apiusersuser-emailbilling-freeze)
            * [DELETE /api/users/{user-email}/billing-freeze](#delete-apiusersuser-emailbilling-freeze)
            * [PUT /api/users/{user-email}/violation-freeze](#put-apiusersuser-emailviolation-freeze)
            * [DELETE /api/users/{user-email}/violation-freeze](#delete-apiusersuser-emailviolation-freeze)
            * [PUT /api/users/{user-email}/legal-freeze](#put-apiusersuser-emaillegal-freeze)
            * [DELETE /api/users/{user-email}/legal-freeze](#delete-apiusersuser-emaillegal-freeze)
            * [DELETE /api/users/{user-email}/billing-warning](#delete-apiusersuser-emailbilling-warning)
            * [GET /api/users/pending-deletion](#get-apiuserspending-deletion)
            * [PATCH /api/users/{user-email}/geofence](#patch-apiusersuser-emailgeofence)
            * [DELETE /api/users/{user-email}/geofence](#delete-apiusersuser-emailgeofence)
            * [PATCH /api/users/{user-email}/activate-account/disable-bot-restriction](#patch-apiusersuser-emailactivate-accountdisable-bot-restriction)
            * [PATCH /api/users/{user-email}/trial-expiration](#patch-apiusersuser-emailtrial-expiration)
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
            * [DELETE /api/projects/{project}/apikeys?name={value}](#delete-apiprojectsprojectapikeysnamevalue)
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
        * [Project API Keys Management](#project-api-keys-management)
            * [GET /api/apikeys/{api-key}](#get-apiapikeysapi-key)
            * [DELETE /api/apikeys/{api-key}](#delete-apiapikeysapi-key)
        * [REST API Keys Management](#rest-api-keys-management)
            * [POST /api/restkeys/{user-email}](#post-apirestkeysuser-email)
            * [PUT /api/restkeys/{api-key}/revoke](#put-apirestkeysapi-keyrevoke)

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
            "publicId": "9551ffef-935c-4d62-9a3b-00d36c411182",
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

#### PUT /api/users/{user-email}/billing-freeze

Freezes a user account so no uploads or downloads may occur.
This is a billing freeze the user can exit automatically by paying their invoice.

#### DELETE /api/users/{user-email}/billing-freeze

Unfreezes a previously billing frozen user account so uploads and downloads may resume.

#### PUT /api/users/{user-email}/violation-freeze

Freezes a user account for violation so no uploads or downloads may occur
User status is also set to Pending Deletion. The user cannot exit this state automatically.

#### DELETE /api/users/{user-email}/violation-freeze

Removes the violation freeze on a user account so uploads and downloads may resume.
User status is set back to Active. This is the only way to exit the violation frozen state.

#### PUT /api/users/{user-email}/legal-freeze

Freezes a user account for legal review so no uploads or downloads may occur
User status is also set to Legal hold. The user cannot exit this state automatically.

#### DELETE /api/users/{user-email}/legal-freeze

Removes the legal freeze on a user account so uploads and downloads may resume.
User status is set back to Active. This is the only way to exit the legal frozen state.


#### DELETE /api/users/{user-email}/billing-warning

Removes the billing warning status from a user's account.

#### GET /api/users/pending-deletion

Returns a limited list of users pending deletion and have no unpaid invoices.
Required parameters: `limit` and `page`.
Example: `/api/users/pending-deletion?limit=10&page=1`

#### PATCH /api/users/{user-email}/geofence

Sets the account level geofence for the user.

Example request:

```json
{
  "region": "US"
}
```

#### DELETE /api/users/{user-email}/geofence

Removes the account level geofence for the user.

#### PATCH /api/users/{user-email}/activate-account/disable-bot-restriction

Disables account bot restrictions by activating the account and restoring its limit values. This is used only for accounts with the PendingBotVerification status.

#### PATCH /api/users/{user-email}/trial-expiration

Updates account free trial expiration date.

Example request:

```json
{
  "trialExpiration": "2024-06-01T00:00:00.000Z"
}
```

or

```json
{
  "trialExpiration": null
}
```

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

#### DELETE /api/projects/{project}/apikeys?name={value}

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
various query parameters (e.g. `usage=5000000&bandwidth=9000000`).

This endpoint also accepts to receive the information as a form in the request body, that is content
type `application/x-www-form-urlencoded` and the header must be specified, otherwise the server
doesn't read the request body.

Using the 0 number means to set them exactly to 0, which is not the same than using the default
value. Default values are applied when they are `nil`. Only the indicated fields support to set the
default value to `nil` using the -1 number.

##### PUT /api/projects/{project-id}/limit?usage={value}

Updates usage limit for a project. The value must be in bytes.

##### PUT /api/projects/{project-id}/limit?bandwidth={value}

Updates bandwidth limit for a project. The value must be in bytes.

##### PUT /api/projects/{project-id}/limit?rate={value}

Updates rate limit for a project.

Accepts -1 to set to `nil`.

##### PUT /api/projects/{project-id}/limit?buckets={value}

Updates number of buckets limit for a project.

Accepts -1 to set to `nil`.

##### PUT /api/projects/{project-id}/limit?burst={value}

Updates burst limit for a project.

Accepts -1 to set to `nil`.

##### PUT /api/projects/{project-id}/limit?segments={value}

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

### Project API Keys Management

#### GET /api/apikeys/{api-key}

Gets information on the given API key.

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
    "name": "My Project"
  },
  "owner": {
    "id": "12345678-1234-1234-1234-123456789abc",
    "fullName": "test user",
    "email": "bob@example.test",
    "paidTier": true
  }
}
```

#### DELETE /api/apikeys/{api-key}

Deletes the given API key.

### REST API Keys Management

#### POST /api/restkeys/{user-email}

Create a REST API key for the user's account associated to the indicated e-mail.

An example of a required request body:

```json
{
    "expiration": "30d20h"
}
```

If `expiration` is empty, the default expiration is applied. Otherwise, the expiration parameter
must have some non-negative value according to https://pkg.go.dev/time#ParseDuration.

#### PUT /api/restkeys/{api-key}/revoke

Revoke the indicated REST API key.
