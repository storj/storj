# satellite/admin

Satellite Admin package provides API endpoints for administrative tasks.

Requires setting `Authorization` header for requests.

## POST /api/user

Adds a new user.

A successful request:

```json
{
    "email": "alice@mail.test",
    "fullName": "Alice Test",
    "password": "password"
}
```

A successful response:

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

A successful update request:

```json
{
    "email": "alice+2@mail.test",
    "shortName": "Al",
    "passwordHash": "1234abcd"
}
```

## GET /api/user/{user-email}

This endpoint returns information about user and their projects.

A successful response:

```json
{
    "user":{
        "id": "12345678-1234-1234-1234-123456789abc",
        "fullName": "Alice Bob",
        "email":"alice@example.test"
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

## POST /api/coupon

Adds a coupon for specific user.

A successful request:

```json
{
    "userId":      "12345678-1234-1234-1234-123456789abc",
    "duration":    2,
    "amount":      3000,
    "description": "promotional coupon (valid for 2 billing cycles)"
}
```

A successful response:
```json
{
    "id": "2fcdbb8f-8d4d-4e6d-b6a7-8aaa1eba4c89"
}
```

## GET /api/coupon/{coupon-id}

Gets a coupon with the specified id.

A successful request:
```json
{
    "id": "2fcdbb8f-8d4d-4e6d-b6a7-8aaa1eba4c89"
}
```

A successful response:
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

## GET /api/project/{project-id}/limit

This endpoint returns information about project limits.

A successful response:

```json
{
    "usage": {
        "amount":"0 B",
        "bytes":0
    },
    "rate":{
        "rps":0
    }
}
```

## POST /api/project/{project-id}/limit?usage={value}

Updates usage limit for a project.

## POST /api/project/{project-id}/limit?rate={value}

Updates rate limit for a project.

## DELETE /api/project/{project-id}

Deletes the project.

## POST /api/project

Adds a project for specific user.

A successful request:

```json
{
    "ownerId": "ca7aa0fb-442a-4d4e-aa36-a49abddae837",
    "projectName": "My Second Project"
}
```

A successful response:

```json
{
    "projectId": "ca7aa0fb-442a-4d4e-aa36-a49abddae837"
}
```
