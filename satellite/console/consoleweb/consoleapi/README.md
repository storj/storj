
Requires setting `Authorization` header for requests.

## API design

### Successful responses

For requests (`PUT`, `POST`, `DELETE`), endpoints should return an empty response body on success. `GET`
requests can return a non-empty body for the resource that we're interacting with. 

### Error responses

When an API endpoint returns a client error (status code 4XX) it returns a JSON error response which contains 2 fields:

* `error`: The error message.
* `detail` (may be empty): Some detail about the returned error.

Example:

```json
{
  "error": "usage for the current month exists",
  "detail": ""
}
```
## Project Management API Endpoints

Example of request
```json
curl  -i -L \
    -H "Accept: application/json" \
    -H 'Authorization: <key>' \
    -X GET \
    "https://satellite.qa.storj.io/api/v0/projects/"
```
#### GET projects/

Get users project. 

A successful response body:

```json
{

}
```
### GET /bucket-rollup/projectID={}&bucket={}&since={Unix Timestamp}&before={Unix Timestamp}
Gets project's single bucket usage by bucket ID
A successful response body:

```json
{

}
```
### GET /bucket-rollups?projectID={}&since={Unix Timestamp}&before={Unix Timestamp}	
Gets project's all buckets usage
A successful response body:

```json
{

}
```
### PUT /create
Creates new Project with given info
Some example request:
```json
{
    "email": "alice+2@mail.test"
}
```
A successful response body:

```json
{

}
```
### PATCH /update
Updates project with given info
Some example request:
```json
{
    "email": "alice+2@mail.test"
}
```
A successful response body:

```json
{

}
```