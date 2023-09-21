# API Docs

**Description:** 

**Version:** `v0`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* Documents
  * [Update Content](#documents-update-content)

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

