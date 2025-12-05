# Migration Guide: Legacy REST API to Generated Private API

This document describes the process for migrating REST API endpoints from the legacy implementation in `satellite/console/consoleweb/server.go` to the generated private API framework.

## Overview

The migration involves moving from manually implemented REST handlers to a code-generated approach that provides:
- Type-safe request/response handling
- Automatic TypeScript client generation
- Consistent error handling
- Better maintainability
- Automatic API documentation

## Migration Steps

### Step 1: Identify the Legacy Endpoint

Locate the endpoint handler in `satellite/console/consoleweb/server.go` that you want to migrate.

Example legacy handler:
```go
func (a *Auth) GetAccount(w http.ResponseWriter, r *http.Request) {
    // Authentication logic
    // Business logic
    // Response formatting
}
```

### Step 2: Check response of the Legacy Handler

Pull out the response structure from the legacy handler. This will be used to define the response type for the generated endpoint (if needed).
Check if request parameters are used, and if so, make sure to reuse them.

### Step 3: Create Service Method

Create a new service method that returns the exact response type that will be used by the generated endpoint.

```go
// In satellite/console/service.go
func (s *Service) GetUserAccount(ctx context.Context) (*UserAccount, api.HTTPError) {
    var err error
    defer mon.Task()(&ctx)(&err)

    user, err := s.getUserAndAuditLog(ctx, "get user account")
    if err != nil {
        return nil, api.HTTPError{
            Status: http.StatusUnauthorized,
            Err:    Error.Wrap(err),
        }
    }

    // other code...

    return &UserAccount{
        ID:       user.ID,
        Email:    user.Email,
        FullName: user.FullName,
    }, nil
}
```

### Step 4: Add Generated Endpoint

Add the endpoint definition to `satellite/console/consoleweb/consoleapi/privateapi/gen/main.go`:

```go
g.Get("/account", &apigen.Endpoint{
    Name:           "Get User account",
    Description:    "Returns existing User",
    GoName:         "GetUserAccount",
    TypeScriptName: "getUserAccount",
    Response:       console.UserAccount{},
})
```

### Step 5: Implement Generated Handler

Create the generated handler method in `satellite/console/consoleweb/consoleapi/privateapi/api.private.gen.go` (this is automatically generated, but you need to ensure the service method is called correctly).

### Step 6: Generate Code

Run the code generation:

```bash
go generate ./satellite/console/consoleweb/consoleapi/privateapi/gen
```

This will generate:
- `api.private.gen.go` - Go handler implementation
- `private.gen.ts` - TypeScript client
- `apidocs.private.gen.md` - API documentation

### Step 7: Update Frontend

Update the frontend code to use the generated TypeScript client:

```typescript
import { AuthManagementHttpApiV0 } from '@/api/private.gen';

const api  = new AuthManagementHttpApiV0();
const userAccount = await api.getUserAccount();
```

### Step 8: Integration

1. Wire up the generated handler in the server setup
2. Deprecate the legacy handler registration

### Step 9: Testing

1. Test the new endpoint functionality
2. Ensure frontend integration works correctly
3. Verify error handling
4. Run existing tests and update as needed

## Tips and Best Practices

1. **Single Responsibility**: Each service method should do one thing well
2. **Type Safety**: Use strongly typed request/response structures
3. **Error Handling**: Let the generated code handle HTTP status codes and error formatting
4. **Documentation**: Use clear names and descriptions for endpoints
5. **Testing**: Test both the service method and the generated endpoint

## Common Pitfalls

1. **Complex Logic in Handlers**: Keep handlers simple - move complex logic to service methods
2. **Authentication**: The generated middleware handles authentication - don't duplicate it
3. **Response Formatting**: Let the generator handle JSON serialization

## Example Migration

See the `GetUserAccount` endpoint as a reference implementation of a successfully migrated endpoint.
