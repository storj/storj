# satellite/admin

Satellite Admin package provides API endpoints for administrative tasks.

Requires setting `Authorization` header for requests.

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
    ]
}
```

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
