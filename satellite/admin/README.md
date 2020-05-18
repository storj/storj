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
    "userId": "12345678-1234-1234-1234-123456789abc"
}
```

## GET /api/user/{useremail}

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
    ]
}
```

## POST /api/user/{useremail}/coupon

Adds a coupon for specific user.

A successful request:

```json
{
    "userid": "12345678-1234-1234-1234-123456789abc",
    "duration": 2,
    "amount": 3000,
    "description": "promotional coupon (valid for 2 billing cycles)"
}
```

A successful response (which lists all coupons for the given user):
```json
{
    "coupons": [{"id": "12345678-1234-1234-1234-123456789abc"}]
}
```

## GET /api/user/{useremail}/coupon

Gets all coupons for specific user.

A successful request:

```json
{
    "userid": "12345678-1234-1234-1234-123456789abc"
}
```

A successful response (which lists all coupons for the given user):
```json
{
    "coupons": [{"id": "12345678-1234-1234-1234-123456789abc"}]
}
```

## GET /api/project/{projectid}/limit

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

## POST /api/project/{projectid}/limit?usage={value}

Updates usage limit for a project.

## POST /api/project/{projectid}/limit?rate={value}

Updates rate limit for a project.

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
