# API Docs

**Description:** 

**Version:** `v0`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* TestAPI
  * [](#testapi-)

<h3 id='testapi-'> (<a href='#list-of-endpoints'>go to full list</a>)</h3>



`POST /api/v0/testapi/{path}`

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

