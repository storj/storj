# Admin Back-Office
<!-- Auto-generate this ToC with https://github.com/ycd/toc -->
<!-- toc -->
- [Admin Back-Office](#admin-back-office)
    * [Setting up the back-office](#setting-up-the-back-office)
        * [Authentication](#authentication)
    * [API](#api)
        * [Guidelines](#guidelines)
            * [Errors](#errors)
            * [Endpoints that return lists](#endpoints-that-return-lists)
                * [Pagination](#pagination)
                * [Sorting](#sorting)
                * [Search](#search)

<!-- tocstop -->

## Setting up the back-office

These environment variables need to be added to the `satellite-admin` container.
```yaml
satellite-admin:
  ####
  ####
  environment:
    ####
    ####
    STORJ_ADMIN_BACK_OFFICE_STATIC_DIR: /var/lib/storj/storj/satellite/admin/back-office/ui
    STORJ_ADMIN_BACK_OFFICE_BYPASS_AUTH: "true"
    ####
    ####
  volumes:
    ####
    ####
    - type: bind
      source: /storj/satellite/admin/back-office/ui
      target: /var/lib/storj/storj/satellite/admin/back-office/ui
      bind: {}
```
For this setup, we can use the back-office via the satellite admin endpoint; `localhost:9080/back-office`. The variable
`STORJ_ADMIN_BACK_OFFICE_BYPASS_AUTH` makes it so that we do not require authentication to access the back-office.

### Authentication
To enable authentication, the back office has to be served through the oauth2-proxy.
```yaml
  oauth2-proxy:
    container_name: oauth2-proxy
    image: quay.io/oauth2-proxy/oauth2-proxy:v7.12.0
    environment:
      OAUTH2_PROXY_PROVIDER: "google"
      OAUTH2_PROXY_CLIENT_ID: "####"
      OAUTH2_PROXY_CLIENT_SECRET: "####"
      OAUTH2_PROXY_HTTP_ADDRESS: "0.0.0.0:4180"
      OAUTH2_PROXY_UPSTREAMS: "http://satellite-admin:8080" # internal port of the satellite-admin container
      OAUTH2_PROXY_REDIRECT_URL: "http://localhost:4180/oauth2/callback"
      OAUTH2_PROXY_COOKIE_SECRET: "####"
      OAUTH2_PROXY_GOOGLE_SERVICE_ACCOUNT_JSON: "/service-account-file.json"
      OAUTH2_PROXY_GOOGLE_ADMIN_EMAIL: "emailofadmin@test.test"
      OAUTH2_PROXY_EMAIL_DOMAINS: "test.test"
      OAUTH2_PROXY_GOOGLE_GROUPS: "admins@test.test,viewers@test.test"
    ports:
      - "4180:4180/tcp"
    hostname: oauth2-proxy
    volumes:
      - "./service-account-file.json:/service-account-file.json"
```
**NOTE**: Do not omit `OAUTH2_PROXY_GOOGLE_GROUPS` not only does it restrict which groups can access the admin, it also
makes sure that the `X-FORWARDED-FOR-GROUPS` header is passed to the admin API.
Also, the service account you use must be granted access to our Google Workspace using the directions [here](https://developers.google.com/workspace/guides/create-credentials#service-account).

The admin container configs should look like this now:
```yaml
satellite-admin:
  ####
  ####
  environment:
    ####
    ####
    STORJ_ADMIN_BACK_OFFICE_USER_GROUPS_ROLE_ADMIN: "admins@test.test" # comma separated for multiple groups
    # optional if you want to test view only access. check the code for other roles.
    STORJ_ADMIN_BACK_OFFICE_USER_GROUPS_ROLE_VIEWER: "viewers@test.test" # comma separated for multiple groups
    STORJ_ADMIN_ALLOWED_OAUTH_HOST: localhost:4180
    ####
    ####
  volumes:
    ####
    ####
    - type: bind
      source: /storj/satellite/admin/back-office/ui
      target: /var/lib/storj/storj/satellite/admin/back-office/ui
      bind: {}
```

Now to access the back-office, go to `localhost:4180/back-office`.

## API

### Guidelines

#### Errors

When an endpoint returns an error must always return  the appropriated HTTP status codes with a body, except that the HTTP specification indicates that the HTTP status code must not have a body.

The body is an object following fields:

```ts
{
	error: string
}
```

`error` field is message which provides a description of the error.

#### Endpoints that return lists

Some of the endpoints that return a list of items require one of this functionalities.
We need to define request parameters and response parameters (for the ones that needs it), so all the endpoints that require these features are consistent across the API.

##### Pagination

We must offer pagination, but we have to consider the performance penalty of using `LIMIT` and `OFFSET`, hence we should try to have model that we can use the [keyset pagination](https://www.cockroachlabs.com/docs/stable/pagination).

The endpoint:
- Accepts three optional query parameters: `cursor`, `direction` and `limit`.

  The server sends the list of items starting from the first record when it doesn't receive any cursor.

  The server applies a default limit when it doesn't receive the `direction` and `limit` parameters.

  The server responds with HTTP status code 422 if `cursor`, `direction`, or `limit` have invalid values.

  `cursor` is an opaque value that the server sends (see next bullet). `dierection` indicates what list of items to retrieve from the cursor, it only accepts two values `next` and `previous`. `limit` is the maximum number of requested items.
- Sends a response body in an object with the following fields:
```ts
{
	data: any[],           // This is an array of the the type
				           // corresponding to the endpoint.
    pagination: {
        cursor: string,      // Opaque value.
        total: number,       // Integer.
        previous: boolean,   // Indicates that there is a previous page of results.
        next: boolean,       // Indicates that there is a next page of results.
    }
}
```

   Clients must send `cursor` when it wants to get the next page of results with the same sort and search parameters used in the previous requests, otherwise the server responds with an HTTP  status code 422 because cursor only applies to exactly the same query.

   `cursor` value is opaque for the clients; the server knows how to interpret it to know if the request contains the correct order and search to return results from the designed cursor.

The trade-off of using a _ketyset pagination_ for having less performance impact on querying the database is that the pagination doesn't support to ask for an arbitrary page, only the next or the previous can be requested.

##### Sorting
We must allow to sort the items by different fields and order (ascendant and descendant).

The endpoints accepts one optional query parameter: `sort-by`. The server applies a default order when it doesn't receive the `sort-by`  parameter.

`sort-by` is a comma-separated list of tuples `<field-name>:<asc|des>`, for example `sort-by=surname:asc,age:des`. Fields' names cannot contain colons and the order of the fields establish the how to sort the items, in the previous example, the server sorts the items by `surname` and after by `age`.

##### Search
We must allow to search items by different fields. When multiple fields are used a logical `AND` is used between the values.

The endpoint accepts one optional query parameter: `filter`. The server doesn't apply any filter if it doesn't receive the `filter` parameter.

`filter` is a comma-separated list of tuples `<field-name>:<value>`, for example `filter=company:storj,name:john`. Field's names cannot contain colons, the order of the fields is irrelevant because a logical `AND` is applied for all of them.
