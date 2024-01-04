# API Docs

**Version:** `v1`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* Settings
  * [Get settings](#settings-get-settings)
* PlacementManagement
  * [Get placements](#placementmanagement-get-placements)
* UserManagement
  * [Get user](#usermanagement-get-user)
* ProjectManagement
  * [Get project](#projectmanagement-get-project)
  * [Update project limits](#projectmanagement-update-project-limits)

<h3 id='settings-get-settings'>Get settings (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets the settings of the service and relevant Storj services settings

`GET /back-office/api/v1/settings/`

**Response body:**

```typescript
{
	admin: 	{
		features: 		{
			account: 			{
				create: boolean
				delete: boolean
				history: boolean
				list: boolean
				projects: boolean
				suspend: boolean
				unsuspend: boolean
				resetMFA: boolean
				updateInfo: boolean
				updateLimits: boolean
				updatePlacement: boolean
				updateStatus: boolean
				updateValueAttribution: boolean
				view: boolean
			}

			project: 			{
				create: boolean
				delete: boolean
				history: boolean
				list: boolean
				updateInfo: boolean
				updateLimits: boolean
				updatePlacement: boolean
				updateValueAttribution: boolean
				view: boolean
				memberList: boolean
				memberAdd: boolean
				memberRemove: boolean
			}

			bucket: 			{
				create: boolean
				delete: boolean
				history: boolean
				list: boolean
				updateInfo: boolean
				updatePlacement: boolean
				updateValueAttribution: boolean
				view: boolean
			}

			dashboard: boolean
			operator: boolean
			signOut: boolean
			switchSatellite: boolean
		}

	}

}

```

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
	projects: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			name: string
			bandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			storageUsed: number
			segmentLimit: number
			segmentUsed: number
		}

	]

}

```

<h3 id='projectmanagement-get-project'>Get project (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets project by ID

`GET /back-office/api/v1/projects/{publicID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	name: string
	description: string
	userAgent: string
	owner: 	{
		id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		fullName: string
		email: string
	}

	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	defaultPlacement: number
	rateLimit: number
	burstLimit: number
	maxBuckets: number
	bandwidthLimit: number
	bandwidthUsed: number
	storageLimit: number
	storageUsed: number
	segmentLimit: number
	segmentUsed: number
}

```

<h3 id='projectmanagement-update-project-limits'>Update project limits (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Updates project limits by ID

`PUT /back-office/api/v1/projects/limits/{publicID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	maxBuckets: number
	storageLimit: number
	bandwidthLimit: number
	segmentLimit: number
	rateLimit: number
	burstLimit: number
}

```

