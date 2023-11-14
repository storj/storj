# API Docs

**Version:** `v1`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* PlacementManagement
  * [Get placements](#placementmanagement-get-placements)
* UserManagement
  * [Get user](#usermanagement-get-user)

<h3 id='placementmanagement-get-placements'>Get placements (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets placement rule IDs and their locations

`GET /back-office/api/v1/placements/`

**Response body:**

```typescript
[
	{
		id: number
		location: string
	}

]

```

<h3 id='usermanagement-get-user'>Get user (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets user by email address

`GET /back-office/api/v1/users/{email}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `email` | `string` |  |

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	fullName: string
	email: string
	paidTier: boolean
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	status: string
	userAgent: string
	defaultPlacement: number
	projectUsageLimits: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			name: string
			storageLimit: number
			storageUsed: number
			bandwidthLimit: number
			bandwidthUsed: number
			segmentLimit: number
			segmentUsed: number
		}

	]

}

```

