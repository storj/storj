# API Docs

**Description:** Interacts with projects

**Version:** `v0`

## Create new Project

Creates new Project with given info

`POST /projects/create`

**Request body:**

```json
{
	name: string
	description: string
	storageLimit: string (Amount of memory formatted as `15 GB`)
	bandwidthLimit: string (Amount of memory formatted as `15 GB`)
	createdAt: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
}

```

**Response body:**

```json
unknown
```

## Update Project

Updates project with given info

`PATCH /projects/update/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

**Request body:**

```json
{
	name: string
	description: string
	storageLimit: string (Amount of memory formatted as `15 GB`)
	bandwidthLimit: string (Amount of memory formatted as `15 GB`)
	createdAt: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
}

```

**Response body:**

```json
{
	id: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
	publicId: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
	name: string
	description: string
	userAgent: 	string
	ownerId: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
	rateLimit: number
	burstLimit: number
	maxBuckets: number
	createdAt: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
	memberCount: number
	storageLimit: string (Amount of memory formatted as `15 GB`)
	bandwidthLimit: string (Amount of memory formatted as `15 GB`)
	userSpecifiedStorageLimit: string (Amount of memory formatted as `15 GB`)
	userSpecifiedBandwidthLimit: string (Amount of memory formatted as `15 GB`)
	segmentLimit: number
	defaultPlacement: number
}

```

## Delete Project

Deletes project by id

`DELETE /projects/delete/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

## Get Projects

Gets all projects user has

`GET /projects/`

**Response body:**

```json
[
	{
		id: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
		publicId: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
		name: string
		description: string
		userAgent: 		string
		ownerId: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
		rateLimit: number
		burstLimit: number
		maxBuckets: number
		createdAt: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
		memberCount: number
		storageLimit: string (Amount of memory formatted as `15 GB`)
		bandwidthLimit: string (Amount of memory formatted as `15 GB`)
		userSpecifiedStorageLimit: string (Amount of memory formatted as `15 GB`)
		userSpecifiedBandwidthLimit: string (Amount of memory formatted as `15 GB`)
		segmentLimit: number
		defaultPlacement: number
	}

]

```

## Get Project's Single Bucket Usage

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

```json
{
	projectID: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
	bucketName: string
	totalStoredData: number
	totalSegments: number
	objectCount: number
	metadataSize: number
	repairEgress: number
	getEgress: number
	auditEgress: number
	since: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
	before: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
}

```

## Get Project's All Buckets Usage

Gets project's all buckets usage

`GET /projects/bucket-rollups`

**Query Params:**

| name | type | elaboration |
|---|---|---|
| `projectID` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |
| `since` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |
| `before` | `string` | Date timestamp formatted as `2006-01-02T15:00:00Z` |

**Response body:**

```json
[
	{
		projectID: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
		bucketName: string
		totalStoredData: number
		totalSegments: number
		objectCount: number
		metadataSize: number
		repairEgress: number
		getEgress: number
		auditEgress: number
		since: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
		before: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
	}

]

```

## Get Project's API Keys

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

```json
{
	apiKeys: 	[
		{
			id: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
			projectId: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
			projectPublicId: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
			userAgent: 			string
			name: string
			createdAt: string (Date timestamp formatted as `2006-01-02T15:00:00Z`)
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

## Create new macaroon API key

Creates new macaroon API key with given info

`POST /apikeys/create`

**Request body:**

```json
{
	projectID: string
	name: string
}

```

**Response body:**

```json
{
	key: string
	keyInfo: unknown
}

```

## Delete API Key

Deletes macaroon API key by id

`DELETE /apikeys/delete/{id}`

**Path Params:**

| name | type | elaboration |
|---|---|---|
| `id` | `string` | UUID formatted as `00000000-0000-0000-0000-000000000000` |

## Get User

Gets User by request context

`GET /users/`

**Response body:**

```json
{
	id: string (UUID formatted as `00000000-0000-0000-0000-000000000000`)
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

