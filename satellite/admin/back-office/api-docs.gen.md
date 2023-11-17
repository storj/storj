# API Docs

**Description:** 

**Version:** `v1`

<h2 id='list-of-endpoints'>List of Endpoints</h2>

* PlacementManagement
  * [Get placements](#placementmanagement-get-placements)

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

