# Generated REST API documentation

These endpoints are not currently used from within the Satellite UI.

Requires setting 'Authorization' header for requests.

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

### Project Management API Endpoints
#### GET /api/v0/projects/
Gets users projects.

!!!WARNING!!! Project ID is used as encryption salt. Please don't send it to anyone. We're going to fix it soon. 

A successful response body:

```json
[
  {
    "id":"cd8d64bd-7457-4661-b88d-2e257bd0d88a",
    "name":"My First Project",
    "description":"",
    "partnerId":"00000000-0000-0000-0000-000000000000",
    "userAgent":null,
    "ownerId":"f0ef7918-c8f0-4a9c-94fe-2260fb2a7877",
    "rateLimit":null,
    "burstLimit":null,
    "maxBuckets":null,
    "createdAt":"2022-04-15T11:38:36.951306+03:00",
    "memberCount":0,
    "storageLimit":"150.00 GB",
    "bandwidthLimit":"150.00 GB",
    "segmentLimit":150000
  }
]
```

#### GET /api/v0/projects/bucket-rollup/projectID={uuid string}&bucket={string}&since={Date Timestamp like '2006-01-02T15:00:00Z'}&before={Date Timestamp like '2006-01-02T15:00:00Z'}
Gets project's single bucket usage by bucket ID.

!!!WARNING!!! Project ID is used as encryption salt. Please don't send it to anyone. We're going to fix it soon.

A successful response body:

```json
{
  "projectID":"f4f2688e-8dae-4401-8ff1-31d9154ba514",
  "bucketName":"bucket",
  "totalStoredData":0.011384611174500622,
  "totalSegments":0.21667078722222222,
  "objectCount":0.10833539361111111,
  "metadataSize":1.2241899478055554e-8,
  "repairEgress":0,
  "getEgress":0,
  "auditEgress":0,
  "since":"2006-01-02T15:00:00Z",
  "before":"2022-04-27T23:59:59Z"
}
```

#### GET /api/v0/projects/bucket-rollups/projectID={uuid string}&since={Date Timestamp like '2006-01-02T15:00:00Z'}&before={Date Timestamp like '2006-01-02T15:00:00Z'}
Gets project's all buckets usage.

!!!WARNING!!! Project ID is used as encryption salt. Please don't send it to anyone. We're going to fix it soon.

A successful response body:

```json
[
  {
    "projectID":"f4f2688e-8dae-4401-8ff1-31d9154ba514",
    "bucketName":"bucket",
    "totalStoredData":0.011384611174500622,
    "totalSegments":0.21667078722222222,
    "objectCount":0.10833539361111111,
    "metadataSize":1.2241899478055554e-8,
    "repairEgress":0,
    "getEgress":0,
    "auditEgress":0,
    "since":"2006-01-02T15:00:00Z",
    "before":"2022-04-27T23:59:59Z"
  }
]
```

#### POST /api/v0/projects/create
Creates new Project with given info.

!!!WARNING!!! Project ID is used as encryption salt. Please don't send it to anyone. We're going to fix it soon.

Request body example:

```json
{
  "name": "new project"
}
```

A successful response body:

```json
{
  "id":"f4f2688e-8dae-4401-8ff1-31d9154ba514",
  "name":"new project",
  "description":"",
  "partnerId":"00000000-0000-0000-0000-000000000000",
  "userAgent":null,
  "ownerId":"f0ef7918-c8f0-4a9c-94fe-2260fb2a7877",
  "rateLimit":null,
  "burstLimit":null,
  "maxBuckets":null,
  "createdAt":"2022-04-27T13:23:59.013381+03:00",
  "memberCount":0,
  "storageLimit":"15 GB",
  "bandwidthLimit":"15 GB",
  "segmentLimit":15000
}
```

#### PATCH /api/v0/projects/update/{uuid string}
Updates project with given info.

!!!WARNING!!! Project ID is used as encryption salt. Please don't send it to anyone. We're going to fix it soon.

Request body example:

```json
{
  "name": "awesome project",
  "description": "random stuff",
  "bandwidthLimit": 1000000000,
  "storageLimit": 1000000000
}
```

A successful response body:

```json
{
  "id":"f4f2688e-8dae-4401-8ff1-31d9154ba514",
  "name":"awesome project",
  "description":"random stuff",
  "partnerId":"00000000-0000-0000-0000-000000000000",
  "userAgent":null,
  "ownerId":"f0ef7918-c8f0-4a9c-94fe-2260fb2a7877",
  "rateLimit":null,
  "burstLimit":null,
  "maxBuckets":null,
  "createdAt":"2022-04-27T13:23:59.013381+03:00",
  "memberCount":0,
  "storageLimit":"1 GB",
  "bandwidthLimit":"1 GB",
  "segmentLimit":15000
}
```

#### DELETE /api/v0/projects/delete/{uuid string}
Deletes project by id.

Note: all the buckets and access grants have to be deleted first and there should not be any usage during current month for paid tier users.

!!!WARNING!!! Project ID is used as encryption salt. Please don't send it to anyone. We're going to fix it soon.

### Macaroon API key API Endpoints
#### POST /api/v0/apikeys/create
Creates new macaroon API key.

!!!WARNING!!! Project ID is used as encryption salt. Please don't send it to anyone. We're going to fix it soon.

Request body example:

```json
{
  "name": "new api key",
  "projectID": "229193d4-0d71-49e8-b9a1-b367b74ed3e3"
}
```

A successful response body:

```json
{
  "key": "13YqdodV5HKad2anjvy6ibtxRtHuD5hUJdsLRpRdQrT4vZ9C2wPJjsQ42L8SeTqMHeW97cwYgxT2FnDjEf3b7Mg5dfUcxm8N157Esq9",
  "keyInfo": {
    "id": "d7a60196-688a-4edc-a561-fd859d04e5a1",
    "projectId": "229193d4-0d71-49e8-b9a1-b367b74ed3e3",
    "partnerId": "00000000-0000-0000-0000-000000000000",
    "userAgent": null,
    "name": "new api key",
    "createdAt": "2022-05-06T14:14:30.076498+03:00"
  }
}
```

### User Endpoints
#### GET /api/v0/users/
Returns User entity by request context.

A successful response body:

```json
{
  "id": "00fe2cef-9412-4c9c-bc42-e57cc062bb7d",
  "fullName": "Test",
  "shortName": "",
  "email": "test@test.com",
  "partnerId": "00000000-0000-0000-0000-000000000000",
  "userAgent": null,
  "projectLimit": 1,
  "isProfessional": false,
  "position": "",
  "companyName": "",
  "employeeCount": "",
  "haveSalesContact": false,
  "paidTier": false,
  "isMFAEnabled": false,
  "mfaRecoveryCodeCount": 0
}
```
