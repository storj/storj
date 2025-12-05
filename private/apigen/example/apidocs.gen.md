# API Docs

**Version:** `v0`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* Documents
  * [Get Documents](#documents-get-documents)
  * [Get One](#documents-get-one)
  * [Get a tag](#documents-get-a-tag)
  * [Get Version](#documents-get-version)
  * [Update Content](#documents-update-content)
* Users
  * [Get Users](#users-get-users)
  * [Create Users](#users-create-users)
  * [Get User's age](#users-get-users-age)
* Projects
  * [Create Projects](#projects-create-projects)

<h3 id='documents-get-documents'>Get Documents (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Get the paths to all the documents under the specified paths

`GET /api/v0/docs/`

**Response body:**

```typescript
[
	{
		id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		pathParam: string
		body: string
		version: 		{
			date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
			number: number
		}

		metadata: 		{
			owner: string
			tags: 			[
unknown
			]

		}

	}

]

```

<h3 id='documents-get-one'>Get One (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Get the document in the specified path

`GET /api/v0/docs/{path}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `path` | `string` |  |

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	pathParam: string
	body: string
	version: 	{
		date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		number: number
	}

	metadata: 	{
		owner: string
		tags: 		[
unknown
		]

	}

}

```

<h3 id='documents-get-a-tag'>Get a tag (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Get the tag of the document in the specified path and tag label 

`GET /api/v0/docs/{path}/tag/{tagName}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `path` | `string` |  |
| `tagName` | `string` |  |

**Response body:**

```typescript
unknown
```

<h3 id='documents-get-version'>Get Version (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Get all the version of the document in the specified path

`GET /api/v0/docs/{path}/versions`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `path` | `string` |  |

**Response body:**

```typescript
[
	{
		date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		number: number
	}

]

```

<h3 id='documents-update-content'>Update Content (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Update the content of the document with the specified path and ID if the last update is before the indicated date

`POST /api/v0/docs/{path}`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `date` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `path` | `string` |  |

**Request body:**

```typescript
{
	content: string
}

```

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	pathParam: string
	body: string
	version: 	{
		date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		number: number
	}

	metadata: 	{
		owner: string
		tags: 		[
unknown
		]

	}

}

```

<h3 id='users-get-users'>Get Users (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Get the list of registered users

`GET /api/v0/users/`

**Response body:**

```typescript
[
	{
		name: string
		surname: string
		email: string
		company: string
		position: string
	}

]

```

<h3 id='users-create-users'>Create Users (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Create users

`POST /api/v0/users/`

**Request body:**

```typescript
[
	{
		name: string
		surname: string
		email: string
		company: string
		position: string
	}

]

```

<h3 id='users-get-users-age'>Get User's age (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Get the user's age

`GET /api/v0/users/age`

**Response body:**

```typescript
{
	day: number
	month: number
	year: number
}

```

<h3 id='projects-create-projects'>Create Projects (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Create projects

`POST /api/v0/projects/`

**Request body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	ownerName: string
}

```

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	ownerName: string
}

```

