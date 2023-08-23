# API Docs

**Description:** Interacts with projects

**Version:** `v0`

**List of endpoints:**
* ProjectManagement
  * [Create new Project](#e-530e988a949a9982f67b860e2581db77)
  * [Update Project](#e-4c85eac5d7ee95019c2749891fb6b6fb)
  * [Delete Project](#e-e490e9360a328ecb5ec9eaf60d410904)
  * [Get Projects](#e-e920a99c4ea7ec5ef4a43c6d2408a118)
  * [Get Project's Single Bucket Usage](#e-ca2c2de1415034f3a51eeb756b8fce9f)
  * [Get Project's All Buckets Usage](#e-bed3c2be3abb8400f944adf3a8f2624c)
  * [Get Project's API Keys](#e-954acbeb1e1a1fff8d97908ef9232ddc)
* APIKeyManagement
  * [Create new macaroon API key](#e-fa1da2b3de26abd107d6aecf6eb39aee)
  * [Delete API Key](#e-bf858e3fbef055f83e996a7f6889eac5)
* UserManagement
  * [Get User](#e-490618f71b154e91e7e446ae28c2d004)

<h2 id='e-530e988a949a9982f67b860e2581db77'>Create new Project</h2>

Creates new Project with given info

`POST /projects/create`

**Request body:**

```typescript
{
	name: string
	description: string
	storageLimit: string // Amount of memory formatted as `15 GB`
	bandwidthLimit: string // Amount of memory formatted as `15 GB`
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
}

```

**Response body:**

```typescript
unknown
```

<h2 id='e-4c85eac5d7ee95019c2749891fb6b6fb'>Update Project</h2>

Updates project with given info

`PATCH /projects/update/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```typescript
{
	name: string
	description: string
	storageLimit: string // Amount of memory formatted as `15 GB`
	bandwidthLimit: string // Amount of memory formatted as `15 GB`
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
}

```

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	publicId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	name: string
	description: string
	userAgent: 	string
	ownerId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	rateLimit: number
	burstLimit: number
	maxBuckets: number
	createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	memberCount: number
	storageLimit: string // Amount of memory formatted as `15 GB`
	bandwidthLimit: string // Amount of memory formatted as `15 GB`
	userSpecifiedStorageLimit: string // Amount of memory formatted as `15 GB`
	userSpecifiedBandwidthLimit: string // Amount of memory formatted as `15 GB`
	segmentLimit: number
	defaultPlacement: number
}

```

<h2 id='e-e490e9360a328ecb5ec9eaf60d410904'>Delete Project</h2>

Deletes project by id

`DELETE /projects/delete/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

<h2 id='e-e920a99c4ea7ec5ef4a43c6d2408a118'>Get Projects</h2>

Gets all projects user has

`GET /projects/`

**Response body:**

```typescript
[
	{
		id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		publicId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		name: string
		description: string
		userAgent: 		string
		ownerId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		rateLimit: number
		burstLimit: number
		maxBuckets: number
		createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		memberCount: number
		storageLimit: string // Amount of memory formatted as `15 GB`
		bandwidthLimit: string // Amount of memory formatted as `15 GB`
		userSpecifiedStorageLimit: string // Amount of memory formatted as `15 GB`
		userSpecifiedBandwidthLimit: string // Amount of memory formatted as `15 GB`
		segmentLimit: number
		defaultPlacement: number
	}

]

```

<h2 id='e-ca2c2de1415034f3a51eeb756b8fce9f'>Get Project's Single Bucket Usage</h2>

Gets project's single bucket usage by bucket ID

`GET /projects/bucket-rollup`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `projectID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `bucket` | `string` |  |
| `since` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |
| `before` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |

**Response body:**

```typescript
{
	projectID: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	bucketName: string
	totalStoredData: number
	totalSegments: number
	objectCount: number
	metadataSize: number
	repairEgress: number
	getEgress: number
	auditEgress: number
	since: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	before: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
}

```

<h2 id='e-bed3c2be3abb8400f944adf3a8f2624c'>Get Project's All Buckets Usage</h2>

Gets project's all buckets usage

`GET /projects/bucket-rollups`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `projectID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `since` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |
| `before` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |

**Response body:**

```typescript
[
	{
		projectID: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
		bucketName: string
		totalStoredData: number
		totalSegments: number
		objectCount: number
		metadataSize: number
		repairEgress: number
		getEgress: number
		auditEgress: number
		since: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		before: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
	}

]

```

<h2 id='e-954acbeb1e1a1fff8d97908ef9232ddc'>Get Project's API Keys</h2>

Gets API keys by project ID

`GET /projects/apikeys/{projectID}`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `search` | `string` |  |
| `limit` | `number` |  |
| `page` | `number` |  |
| `order` | `number` |  |
| `orderDirection` | `number` |  |

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `projectID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Response body:**

```typescript
{
	apiKeys: 	[
		{
			id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			projectId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			projectPublicId: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
			userAgent: 			string
			name: string
			createdAt: string // Date timestamp formatted as `2006-01-02T15:00:00Z`
		}

	]

	search: string
	limit: number
	order: number
	orderDirection: number
	offset: number
	pageCount: number
	currentPage: number
	totalCount: number
}

```

<h2 id='e-fa1da2b3de26abd107d6aecf6eb39aee'>Create new macaroon API key</h2>

Creates new macaroon API key with given info

`POST /apikeys/create`

**Request body:**

```typescript
{
	projectID: string
	name: string
}

```

**Response body:**

```typescript
{
	key: string
	keyInfo: unknown
}

```

<h2 id='e-bf858e3fbef055f83e996a7f6889eac5'>Delete API Key</h2>

Deletes macaroon API key by id

`DELETE /apikeys/delete/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

<h2 id='e-490618f71b154e91e7e446ae28c2d004'>Get User</h2>

Gets User by request context

`GET /users/`

**Response body:**

```typescript
{
	id: string // UUID formatted as `00000000-0000-0000-0000-000000000000`
	fullName: string
	shortName: string
	email: string
	userAgent: 	string
	projectLimit: number
	isProfessional: boolean
	position: string
	companyName: string
	employeeCount: string
	haveSalesContact: boolean
	paidTier: boolean
	isMFAEnabled: boolean
	mfaRecoveryCodeCount: number
}

```

