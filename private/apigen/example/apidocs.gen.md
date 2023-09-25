# API Docs

**Description:** 

**Version:** `v0`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* Documents
  * [Get Documents](#documents-get-documents)
  * [Get One](#documents-get-one)
  * [Get a tag](#documents-get-a-tag)
  * [Get Version](#documents-get-version)
  * [Update Content](#documents-update-content)

<h3 id='documents-get-documents'>Get Documents (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Get the paths to all the documents under the specified paths

`GET /api/v0/docs/`

**Response body:**

```typescript
[
	{
		id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		path: string
		date: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		metadata: 		{
			owner: string
			tags: 			[
unknown
			]

		}

		last_retrievals: 		[
			{
				user: string
				when: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
			}

		]

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
}

```

