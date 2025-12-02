# API Docs

**Version:** `v1`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* Settings
  * [Get settings](#settings-get-settings)
* PlacementManagement
  * [Get placements](#placementmanagement-get-placements)
* ProductManagement
  * [Get products](#productmanagement-get-products)
* UserManagement
  * [Get freeze event types](#usermanagement-get-freeze-event-types)
  * [Get user kinds](#usermanagement-get-user-kinds)
  * [Get user statuses](#usermanagement-get-user-statuses)
  * [Search users](#usermanagement-search-users)
  * [Get user](#usermanagement-get-user)
  * [Get user](#usermanagement-get-user)
  * [Update user](#usermanagement-update-user)
  * [Disable user](#usermanagement-disable-user)
  * [Freeze/Unfreeze User](#usermanagement-freezeunfreeze-user)
  * [Toggle MFA](#usermanagement-toggle-mfa)
  * [Create Rest Key](#usermanagement-create-rest-key)
* ProjectManagement
  * [Get project statuses](#projectmanagement-get-project-statuses)
  * [Get project](#projectmanagement-get-project)
  * [Get project buckets](#projectmanagement-get-project-buckets)
  * [Update bucket](#projectmanagement-update-bucket)
  * [Get bucket state](#projectmanagement-get-bucket-state)
  * [Update project](#projectmanagement-update-project)
  * [Disable project](#projectmanagement-disable-project)
  * [Update project limits](#projectmanagement-update-project-limits)
  * [Update project entitlements](#projectmanagement-update-project-entitlements)
* Search
  * [Search users or projects](#search-search-users-or-projects)
* ChangeHistory
  * [Get change history](#changehistory-get-change-history)

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
				createRestKey: boolean
				delete: boolean
				markPendingDeletion: boolean
				history: boolean
				list: boolean
				projects: boolean
				search: boolean
				suspend: boolean
				unsuspend: boolean
				disableMFA: boolean
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
				markPendingDeletion: boolean
				history: boolean
				list: boolean
				updateInfo: boolean
				updateLimits: boolean
				updatePlacement: boolean
				updateValueAttribution: boolean
				setEntitlements: boolean
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

<h3 id='productmanagement-get-products'>Get products (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets all defined product definitions

`GET /back-office/api/v1/products/`

**Response body:**

```typescript
[
	{
		productID: number
		productName: string
		storageMBMonthCents: string
		egressMBCents: string
		segmentMonthCents: string
		egressDiscountRatio: string
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

<h3 id='usermanagement-search-users'>Search users (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Search users by email or name. Results are limited to 100 users.

`GET /back-office/api/v1/users/`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `term` | `string` |  |

**Response body:**

```typescript
[
	{
		id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		fullName: string
		email: string
		kind: 		{
			value: number
			name: string
			hasPaidPrivileges: boolean
		}

		status: 		{
			name: string
			value: number
		}

		createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
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
			userSetBandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			userSetStorageLimit: number
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
	mfaEnabled: boolean
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
			userSetBandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			userSetStorageLimit: number
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
	mfaEnabled: boolean
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
	trialExpiration: string
	userAgent: string
	projectLimit: number
	storageLimit: number
	bandwidthLimit: number
	segmentLimit: number
	reason: string
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
			userSetBandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			userSetStorageLimit: number
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
	mfaEnabled: boolean
}

```

<h3 id='usermanagement-disable-user'>Disable user (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Disables user by ID. User can only be disabled if they have no active projects and pending invoices. It can also set status to pending deletion.

`PUT /back-office/api/v1/users/{userID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	setPendingDeletion: boolean
	reason: string
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
			userSetBandwidthLimit: number
			bandwidthUsed: number
			storageLimit: number
			userSetStorageLimit: number
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
	mfaEnabled: boolean
}

```

<h3 id='usermanagement-freezeunfreeze-user'>Freeze/Unfreeze User (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Freeze or unfreeze a user account

`PUT /back-office/api/v1/users/{userID}/freeze-events`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	action: string
	type: number
	reason: string
}

```

<h3 id='usermanagement-toggle-mfa'>Toggle MFA (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Toggles MFA for a user. Only disabling is supported.

`PUT /back-office/api/v1/users/{userID}/mfa`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	reason: string
}

```

<h3 id='usermanagement-create-rest-key'>Create Rest Key (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Creates a rest API key a user

`POST /back-office/api/v1/users/rest-keys/{userID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `userID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	expiration: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	reason: string
}

```

**Response body:**

```typescript
string
```

<h3 id='projectmanagement-get-project-statuses'>Get project statuses (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets available project statuses

`GET /back-office/api/v1/projects/statuses`

**Response body:**

```typescript
[
	{
		name: string
		value: number
	}

]

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
	rateLimitHead: number
	burstLimitHead: number
	rateLimitGet: number
	burstLimitGet: number
	rateLimitPut: number
	burstLimitPut: number
	rateLimitDelete: number
	burstLimitDelete: number
	rateLimitList: number
	burstLimitList: number
	maxBuckets: number
	bandwidthLimit: number
	userSetBandwidthLimit: number
	bandwidthUsed: number
	storageLimit: number
	userSetStorageLimit: number
	storageUsed: number
	segmentLimit: number
	segmentUsed: number
	status: unknown
	entitlements: unknown
}

```

<h3 id='projectmanagement-get-project-buckets'>Get project buckets (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets a project's buckets

`GET /back-office/api/v1/projects/{publicID}/buckets`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `search` | `string` |  |
| `page` | `string` |  |
| `limit` | `string` |  |
| `since` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |
| `before` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Response body:**

```typescript
{
	items: 	[
		{
			name: string
			userAgent: string
			placement: string
			storage: number
			egress: number
			segmentCount: number
			createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		}

	]

	limit: number
	offset: number
	pageCount: number
	currentPage: number
	totalCount: number
}

```

<h3 id='projectmanagement-update-bucket'>Update bucket (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Updates a bucket's user agent, and placement if the bucket is empty

`PATCH /back-office/api/v1/projects/{publicID}/buckets/{bucketName}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `bucketName` | `string` |  |

**Request body:**

```typescript
{
	userAgent: string
	placement: number
	reason: string
}

```

<h3 id='projectmanagement-get-bucket-state'>Get bucket state (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Gets a bucket's state that is not stored in the buckets table and requires additional queries.

`GET /back-office/api/v1/projects/{publicID}/buckets/{bucketName}/state`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `bucketName` | `string` |  |

**Response body:**

```typescript
{
	empty: boolean
}

```

<h3 id='projectmanagement-update-project'>Update project (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Updates project name, user agent and default placement by ID

`PATCH /back-office/api/v1/projects/{publicID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	name: string
	description: string
	userAgent: string
	status: number
	defaultPlacement: number
	reason: string
}

```

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
	rateLimitHead: number
	burstLimitHead: number
	rateLimitGet: number
	burstLimitGet: number
	rateLimitPut: number
	burstLimitPut: number
	rateLimitDelete: number
	burstLimitDelete: number
	rateLimitList: number
	burstLimitList: number
	maxBuckets: number
	bandwidthLimit: number
	userSetBandwidthLimit: number
	bandwidthUsed: number
	storageLimit: number
	userSetStorageLimit: number
	storageUsed: number
	segmentLimit: number
	segmentUsed: number
	status: unknown
	entitlements: unknown
}

```

<h3 id='projectmanagement-disable-project'>Disable project (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Disables a project by ID. It can also set status to pending deletion.

`PUT /back-office/api/v1/projects/{publicID}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	setPendingDeletion: boolean
	reason: string
}

```

<h3 id='projectmanagement-update-project-limits'>Update project limits (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Updates project limits by ID

`PATCH /back-office/api/v1/projects/{publicID}/limits`

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
	userSetStorageLimit: number
	userSetBandwidthLimit: number
	rateLimitHead: number
	burstLimitHead: number
	rateLimitGet: number
	burstLimitGet: number
	rateLimitPut: number
	burstLimitPut: number
	rateLimitDelete: number
	burstLimitDelete: number
	rateLimitList: number
	burstLimitList: number
	reason: string
}

```

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
	rateLimitHead: number
	burstLimitHead: number
	rateLimitGet: number
	burstLimitGet: number
	rateLimitPut: number
	burstLimitPut: number
	rateLimitDelete: number
	burstLimitDelete: number
	rateLimitList: number
	burstLimitList: number
	maxBuckets: number
	bandwidthLimit: number
	userSetBandwidthLimit: number
	bandwidthUsed: number
	storageLimit: number
	userSetStorageLimit: number
	storageUsed: number
	segmentLimit: number
	segmentUsed: number
	status: unknown
	entitlements: unknown
}

```

<h3 id='projectmanagement-update-project-entitlements'>Update project entitlements (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Updates project entitlements by ID. Only one entitlement can be updated at a time.

`PATCH /back-office/api/v1/projects/{publicID}/entitlements`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `publicID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	newBucketPlacements: 	[
number
	]

	computeAccessToken: string
	placementProductMappings: unknown
	reason: string
}

```

**Response body:**

```typescript
{
	newBucketPlacements: 	[
string
	]

	computeAccessToken: string
	placementProductMappings: unknown
}

```

<h3 id='search-search-users-or-projects'>Search users or projects (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Searches for users by email or name and projects by ID. Results are limited to 100 users.

`GET /back-office/api/v1/search/`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `term` | `string` |  |

**Response body:**

```typescript
{
	project: unknown
	accounts: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			fullName: string
			email: string
			kind: 			{
				value: number
				name: string
				hasPaidPrivileges: boolean
			}

			status: 			{
				name: string
				value: number
			}

			createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		}

	]

}

```

<h3 id='changehistory-get-change-history'>Get change history (<a href='#list-of-endpoints'>go to full list</a>)</h3>

Retrieves change history for users, projects and buckets. If the exact parameter is `true`, this wouldfetch changes strictly on the user, project or bucket. It'll do otherwise if it's `false`.

`GET /back-office/api/v1/changehistory/`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `exact` | `string` |  |
| `itemType` | `string` |  |
| `id` | `string` |  |

**Response body:**

```typescript
[
	{
		id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		userID: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		projectID: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		bucketName: string
		adminEmail: string
		itemType: string
		reason: string
		operation: string
		changes: unknown
		timestamp: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	}

]

```

