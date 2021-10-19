# Geofencing support

## Abstract

This document proposes a new way to restrict storage of data based on specific constraints. And introduces a simple IP-based geo-fencing restriction as the first implementation.

## Background

Storagenodes are selected for each segment to store new data or replicate existing data to more storagenodes. Today this selection is based on randomization but can restrict the result set to use nodes only from different IPV4 `/24` subnets.

For some specific use-cases it is required to define more restrictions and parameters for node selection. A typical case is when data should be placed in a certain geographic region (like US or EU) due to legal requirements.

## Design

The problem has two parts:
 1. We need to improve the existing mechanism (segment recovery, segment creation) to support node selection constraints 
 2. We need to implement a geo-fencing node selection constraint and maintain regional information for each node.

### Storing constraints

First, we need to set up and store the constraints.

The constraint will be defined on bucket level. During the object creation, the constraint will be used to select nodes for segments.

The constraint should be saved both on bucket level (using as a default for every new segments) and segment level. During repair and graceful-exit processes we have access only to the segment information therefore we will save the placement constraint information to the segment table too.

As the segment table can be huge, the size of the placement information should be minimal. It will be stored on two bytes `INT(2)`. The exact representation can be an incrementing number, meaningful bitmask (different bit prefix may have different meanings) or ASCII iso codes.

**Object creation**:

During the segment creation (`BeginSegment`) the bucket information is not directly available (without one additional DB call) therefore the placement information should be added to the `StreamId`. Stream id is a raw binary that is part of the response of the `BeginObject` call. It is created by the *satellite* and any new placement information constraint can be added. 

`BeginObject` call already checks the existence of the bucket. It can be improved to get the metadata from the bucket instead of just checking the existence...

**Repair** 

It also requires the information of the placement as new nodes may be allocated during the recovery process. The segment loop in the recovery process selects segments to check the right amount of replicas. This can be extended by adding the placement constraints to the segments table as well. This means that the placement information should be persisted during the `BeginSegment` call.
 
 **Graceful exit**
 
Graceful exit uses a slightly different approach for node selection, it directly queries the database instead of using an in-memory cache. Here the placement constraint can either be added to the WHERE clause ot the SQL query or in-memory filtering can be executed after requesting the nodes.

 
### IP based geo-fencing

First, a simple IP-based geofencing constraint will be implemented. While IP geolocation databases may have correctness issues (especially with IPv6) it is good enough for the first implementation. Later the constraint can be further improved to filter out nodes where identification is not reliable or to use more strict rules.

To implement IP-based constrain we need to store the geolocation information (`country`) during the node check-in. Today the node information is updated during checking, which can be extended with identifying the regional information based on a geo-ip database.

In case of the region can not be identified we can exclude the node from geo-fenced node selection.

**Alternatives:**
 * `ip -> country` mapping can be calculated during the database read (as it should be easy to cache), but saving to the database can be better:
     * It makes it easier to do statistics from the node table
     * It makes it more resilient (in case of only GeoIP endpoint is used)
     * It follows the existing practice `last_net` is already saved even if it can be derived from `last_ip_port`

## Not a scope / future plans

### Moving storage nodes between countries

Some existing storagenode may be moved between countries. In this case, some of the restricted data may be moved out from the restricted region. The first implementation won't support this case: node selection constraint will be used only during the segment creation or segment recovery.

However the constraint will be saved to the segment database, therefore it will be possible ti further improve the recovery process to check the right placement of a segment.

### IPV6 and advanced country identification

Current geo-fencing will be based on best-effort IPv4 resolution. Later the country identification code can be improved.

## Implementation

### Distributing Maxmind on Kubernetes

Currently, `linksharing` is one of the few components of our infrastructure that uses the GeoIP dataset. Unfortunately, `linksharing` and the `satellite` processes run in slightly different ways. While `linksharing` has its own set of hosts that are managed by [Ansible](https://github.com/storj/infra/blob/49f5b0cface6a4e89513f2d401c3e5344a064f92/ansible/playbooks/linksharing.yaml), `satellites` run in Kubernetes. Since the bulk of our containers are public, shipping the database in an existing container isn't safe as we can unintentionally expose the Maxmind data beyond our company.

A safe way for us to expose this data to the application is through the use of an init container. The init container is responsible for downloading the latest version of the GeoIP database into a shared `emptyDir: {}` file system for the satellite-api to consume.

Alternatively, we can have a CronJob that does this independently of the satellite. The downside to this approach is that we would need to support ReadWriteOnce / ReadOnlyMany volumes. Since few storage drivers support ReadOnlyMany, we would be coupled to GCP storage drivers until others catch up.

### Required API Modifications

To make life easy, we plan to add a new administrative API endpoint to enable geofencing for a bucket. The API should follow some of the conventions that are already detailed [here](https://github.com/storj/storj/blob/main/satellite/admin/README.md).

- `POST /api/projects/{project-id}/bucket/{bucket-name}/geofence` enables geofencing for a specific bucket within a project.
- `DELETE /api/projects/{project-id}/bucket/{bucket-name}/geofence` disables geofencing for a specific bucket within a project.
- `GET /api/projects/{project-id}/bucket/{bucket-name}/geofence` can return the current geofencing state.

### Required db changes

 1. Add `country` field to the nodes table 
 2. Add `placement` field to the `bucket_metainfos` table `int(2)` 
 3. Add `placement` field to the `segments` table `int(2)` (metainfo)
 
### Required protocol change

The implementation doesn't require any high-level protocol change. The placement constraint could be added to the `SatStreamId` which is shared with the client as a byte array. The client sends the stream id back to the satellite during segment creation where the bucket constraint can be parsed from the stream id and used to select the right nodes for the segments.

## References

* [Initial discussion](https://github.com/storj/storj/issues/694)
