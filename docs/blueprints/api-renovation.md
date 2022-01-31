# API Renovation

## Abstract

This blueprint describes the design and implementation details of the new API system. Long term, pre-existing APIs will be modified to utilize this system, in particular the Console API.

The new API is designed to

* make it very easy to add new endpoints easily with minimal manually-written code
* easily version the API so that new features can be added while supporting backwards compatibility
* authenticate and handle external requests so that developers can use this API from anywhere

## Background

The topics we need to address in this document can be broken into a few categories:

* Authentication
* Versioning
* Generation

### Authentication

The Console API is currently authenticated via a cookie that is inserted when a customer logs in. Although this approach is sufficient for using the Satellite GUI, it is suboptimal for a developer who wishes to programmatically perform operations on resources relating to his account. Therefore, both cookie-based authentication and API-key authentication must be supported in the new API.

### Versioning

Versioning must be implemented from the beginning to ensure compatibility with previous implementations of the Account Management API. When performing API calls, the API version may be specified by a unique path segment, such as `v1` in `api.storj.io/v1/projects/{id}`.

### Generation

The most tedious parts of our current Console API are writing boilerplate code to handle requests in TypeScript (client) and Go (server). The TypeScript code generally inserts some arguments into a JSON body and sends this to a specific endpoint on the server. [TypeScript example for token deposit endpoint](https://github.com/storj/storj/blob/main/web/satellite/src/api/payments.ts#L242-L257).

On the server side, we typically authenticate the request, parse the JSON body into a Go struct, and pass it to a service method for more significant processing. [Go example for token deposit endpoint](https://github.com/storj/storj/blob/main/satellite/console/consoleweb/consoleapi/payments.go#L261-L320).

Much of this boilerplate code is simple enough to be automatically generated as long as we have a source of truth that specifies the endpoints, request types, response types, etc. Then, all we have to do is write the service-level logic on the back end and the UI-level logic on the front end for everything to work!

## Design

The design for the new API system consists of a Go and TypeScript code generator along with API definitions from which the code is generated.

Feature list:

* __Cookie-based authentication__: This is required for the Satellite GUI to interact with the API.
* __API key-based authentication__: This is required for programmatic access to the API.
* __Disable cookie-based or API key-based auth individually__: Certain operations should be restricted to only one authentication type. For example, highly sensitive operations such as MFA configuration should only be performed by a user who has been authenticated through the Satellite GUI in case the user's API key has been compromised.
* __Versioning__: We should be able to update endpoints easily and with minimal manual changes while maintaining old code.
* __Generated Go code__: The new API system should be able to generate boilerplate Go code for the server side of the API.
* __Field validations__: Automatically generating code to handle validation of request fields minimizes the amount of code that must be manually written by developers. This is especially useful when validation types are used more than once as it reduces redundancy.
* __Wide variety of types__: The new API system should support input/output fields that are primitives, slices/arrays, or simple structures.
* __Path parameters__: Path parameter support is necessary to allow users to navigate to resources whose path may not be known at the time of API design. These parameters should be validated and their expected types should be specified in the API definition.
* __Query parameters__: Query parameter support is necessary to allow users to specify additional information about the resource being requested. These parameters should be validated and their expected types should be specified in the API definition.
* __Optional arguments__: An endpoint may assume default values for unspecified parameters to reduce the amount of information that must be specified by the user.
* __Generated type-safe TS code__: The new API system should be able to generate boilerplate TypeScript code for the client side of the API.
* __Generated documentation__: Developers must have information pertaining to proper use of our API in order to use it most effectively.

## Rationale

We [experimented a little](https://github.com/xaresys/storj-api-gen) with writing our own custom code generator. The process was straightforward and gave us an idea of how to proceed with API generation. Although there are pre-existing API code generators, we will build a custom one for the sake of flexibility and ease of maintenance.

## Implementation

The following is a conceptual definition for an API only implementing the Satellite Admin API endpoint responsible for returning user information:
```Go
func main() {
    adminAPI := apigen.NewAPI("Satellite Admin API").Version(0)
    adminAPI.Get("/users/{email: string}")
        .Name("Get User Info")
        .MethodName("GetUserInfo")
        .NoCookieAuth()
        .Response(admin.UserInfo)
        .Validate("email", IsEmailAddress)
}
```
Among the generated code should be Go interfaces defining the required service-level methods that the boilerplate code calls after processing. For example, the following interface corresponds to the previous API definition:
```Go
type SatelliteAdminAPIV0Interface interface {
	GetUserInfo(email string) (*admin.UserInfo, APIError)
}
```

## Wrapup

The User Growth team is responsible for archiving this blueprint upon completion.

Before this document is archived, tickets must be created to publish the API documentation and to migrate the old Console API to use the new Account Management API where possible.
