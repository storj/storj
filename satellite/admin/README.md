# satellite/admin

Satellite Admin package provides API endpoints for administrative tasks.

Requires setting `Authorization` header for requests.

## POST /api/user

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

## PUT /api/user/{user-email}

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

## GET /api/user/{user-email}

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
    ],
    "coupons": [
        {
            "id":          "2fcdbb8f-8d4d-4e6d-b6a7-8aaa1eba4c89",
            "userId":      "12345678-1234-1234-1234-123456789abc",
            "duration":    2,
            "amount":      3000,
            "description": "promotional coupon (valid for 2 billing cycles)",
            "type":        0,
            "status":      0,
            "created":     "2020-05-19T00:34:13.265761+02:00"
        }
    ]
}
```

## DELETE /api/user/{user-email}

Deletes the user.

## POST /api/coupon

Adds a coupon for specific user.

An example of a required request body:

```json
{
    "userId":      "12345678-1234-1234-1234-123456789abc",
    "duration":    2,
    "amount":      3000,
    "description": "promotional coupon (valid for 2 billing cycles)"
}
```

A successful response body:

```json
{
    "id": "2fcdbb8f-8d4d-4e6d-b6a7-8aaa1eba4c89"
}
```

## GET /api/coupon/{coupon-id}

Gets a coupon with the specified id.

A successful response body:

```json
{
    "id":          "2fcdbb8f-8d4d-4e6d-b6a7-8aaa1eba4c89",
    "userId":      "12345678-1234-1234-1234-123456789abc",
    "duration":    2,
    "amount":      3000,
    "description": "promotional coupon (valid for 2 billing cycles)",
    "type":        0,
    "status":      0,
    "created":     "2020-05-19T00:34:13.265761+02:00"
}
```

## DELETE /api/coupon/{coupon-id}

Deletes the specified coupon.

## GET /api/project/{project-id}/usage

This endpoint returns whether the project has outstanding usage or not.

A project with not usage returns status code 200 and `{"result":"no project usage exist"}`.

## GET /api/project/{project-id}/limit

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
  }
}
```

## POST /api/project/{project-id}/limit?usage={value}

Updates usage limit for a project.

## POST /api/project/{project-id}/limit?bandwidth={value}

Updates bandwidth limit for a project.

## POST /api/project/{project-id}/limit?rate={value}

Updates rate limit for a project.

## POST /api/project/{project-id}/limit?buckets={value}

Updates bucket limit for a project.

## GET /api/project/{project-id}

Gets the common information about a project.

## PUT /api/project/{project-id}

Updates project name or description.

```json
{
    "projectName": "My new Project Name",
    "description": "My new awesome description!"
}
```

## DELETE /api/project/{project-id}

Deletes the project.

## POST /api/project

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
    "projectId": "ca7aa0fb-442a-4d4e-aa36-a49abddae837"
}
```
