# Generated Private Console API

The API defined in this package is used internally by the Satellite UI to communicate with the backend services. This is a private API that is not exposed to external users and is only used for internal UI operations.

This API is being migrated from the legacy REST handlers in `satellite/console/consoleweb/server.go` to use the code generation framework for better maintainability and consistency.

## Available Endpoints

Generated detailed documentation for each endpoint implemented can be found [here](apidocs.private.gen.md).

## Usage

This API is used internally by the Satellite UI and requires cookie-based authentication. The endpoints are called automatically by the frontend components.

Example of internal usage:
```typescript
import { AuthManagementHttpApiV0 } from '@/api/private.gen';

// Create an instance of the API client.
const api = new AuthManagementHttpApiV0();
// Get user account information.
const userAccount = await api.getUserAccount();
```

## Authentication

This API uses cookie-based authentication only (no API keys). All requests must be authenticated through the standard Satellite UI login process.

## Response Format

All successful responses return JSON data directly without additional wrapping:

```json
{
  "id": "user-id",
  "email": "user@example.com",
  "fullName": "User Name"
}
```

## Error responses

When an API endpoint returns an error (status code 4XX or 5XX), it contains a JSON error response with an error field:

```json
{
  "error": "authentication required"
}
```

## Migration Status

This package is part of an ongoing migration from legacy REST handlers to generated API endpoints. New endpoints should be added here using the API generator framework defined in `gen/main.go`.
