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
