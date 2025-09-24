# API Docs

**Version:** `v1`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* Settings
  * [Get settings](#settings-get-settings)
* PlacementManagement
  * [Get placements](#placementmanagement-get-placements)
* UserManagement
  * [Get freeze event types](#usermanagement-get-freeze-event-types)
  * [Get user kinds](#usermanagement-get-user-kinds)
  * [Get user statuses](#usermanagement-get-user-statuses)
  * [Get user](#usermanagement-get-user)
  * [Get user](#usermanagement-get-user)
  * [Update user](#usermanagement-update-user)
  * [Delete user](#usermanagement-delete-user)
  * [Freeze User](#usermanagement-freeze-user)
  * [Unfreeze User](#usermanagement-unfreeze-user)
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
				search: boolean
				suspend: boolean
				unsuspend: boolean
				resetMFA: boolean
				updateLimits: boolean
				updatePlacement: boolean
				updateStatus: boolean
				updateEmail: boolean
				updateKind: boolean
				updateName: boolean
				updateUserAgent: boolean
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

<h3 id='usermanagement-get-freeze-event-types'>Get freeze event types (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets account freeze event types

`GET /back-office/api/v1/users/freeze-event-types`

**Response body:**

```typescript
[
	{
		name: string
		value: number
	}

]

```

<h3 id='usermanagement-get-user-kinds'>Get user kinds (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets available user kinds

`GET /back-office/api/v1/users/kinds`

**Response body:**

```typescript
[
	{
		value: number
		name: string
		hasPaidPrivileges: boolean
	}

]

```

<h3 id='usermanagement-get-user-statuses'>Get user statuses (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets available user statuses

`GET /back-office/api/v1/users/statuses`

**Response body:**

```typescript
[
	{
		name: string
		value: number
	}

]

```

<h3 id='usermanagement-get-user'>Get user (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets user by email address

`GET /back-office/api/v1/users/email/{email}`

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
	kind: 	{
		value: number
		name: string
		hasPaidPrivileges: boolean
	}

	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	upgradeTime: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	status: 	{
		name: string
		value: number
	}

	userAgent: string
	defaultPlacement: number
	projects: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			name: string
			active: boolean
			bandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			storageUsed: number
			segmentLimit: number
			segmentUsed: number
		}

	]

	projectLimit: number
	storageLimit: number
	bandwidthLimit: number
	segmentLimit: number
	freezeStatus: unknown
	trialExpiration: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	hasUnpaidInvoices: boolean
}

```

<h3 id='usermanagement-get-user'>Get user (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets user by ID

`GET /back-office/api/v1/users/{userID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	fullName: string
	email: string
	kind: 	{
		value: number
		name: string
		hasPaidPrivileges: boolean
	}

	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	upgradeTime: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	status: 	{
		name: string
		value: number
	}

	userAgent: string
	defaultPlacement: number
	projects: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			name: string
			active: boolean
			bandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			storageUsed: number
			segmentLimit: number
			segmentUsed: number
		}

	]

	projectLimit: number
	storageLimit: number
	bandwidthLimit: number
	segmentLimit: number
	freezeStatus: unknown
	trialExpiration: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	hasUnpaidInvoices: boolean
}

```

<h3 id='usermanagement-update-user'>Update user (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Updates user info by ID. Limit updates will cascade to all projects of the user.Updating user kind to NFR or Paid without providing limits will set the limits to kind defaults.

`PATCH /back-office/api/v1/users/{userID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	email: string
	name: string
	kind: number
	status: number
	trialExpiration: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	userAgent: string
	projectLimit: number
	storageLimit: number
	bandwidthLimit: number
	segmentLimit: number
}

```

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	fullName: string
	email: string
	kind: 	{
		value: number
		name: string
		hasPaidPrivileges: boolean
	}

	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	upgradeTime: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	status: 	{
		name: string
		value: number
	}

	userAgent: string
	defaultPlacement: number
	projects: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			name: string
			active: boolean
			bandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			storageUsed: number
			segmentLimit: number
			segmentUsed: number
		}

	]

	projectLimit: number
	storageLimit: number
	bandwidthLimit: number
	segmentLimit: number
	freezeStatus: unknown
	trialExpiration: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	hasUnpaidInvoices: boolean
}

```

<h3 id='usermanagement-delete-user'>Delete user (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Deletes user by ID. User can only be deleted if they have no active projects and pending invoices.

`DELETE /back-office/api/v1/users/{userID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

<h3 id='usermanagement-freeze-user'>Freeze User (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Freeze a user account

`POST /back-office/api/v1/users/{userID}/freeze-events`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	type: number
}

```

<h3 id='usermanagement-unfreeze-user'>Unfreeze User (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Unfreeze a user account

`DELETE /back-office/api/v1/users/{userID}/freeze-events`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

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

