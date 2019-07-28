# Value Attribution

### Brandon Iglesias, Dennis Coyle


## Definitions

*Attribution* - _Acknowledgment for establishing the relationship between Storj and a customer_


## OVERVIEW

When our partners bring data to the Tardigrade network we want the ability to attach attribution for the data. 

Our partners will have connectors that their customers will use to store data on the Tardigrade network. When one of their customers uses the connector to store data in a bucket on the Tardigrade network, we want to give that partner attribution for the data that is stored in it.


## GOALS

* A generic library that can be used in all  Golang third party connectors
* Data attribution on a per bucket basis
* Ability to calculate how much data was stored in the bucket for a given amount of time. 
* Ability to calculate attributed data bandwidth on the Storj Network


## NON GOALS

* Attributions per object
* Multiple attributions per bucket
* Attributions per project
* Attributions per satellite account


## SCENARIOS

* A client points partner connector A to a bucket with NO data in it.
	* The partner receives the attribution for all of the data that the client stores and retrieves from it. 
	* The bucket must be empty when the partner connector is connected to it. 
* A client points partner connector A to an existing bucket that NO OTHER connector has pointed to but has data in it. 
	* The partner does NOT receive attribution for data stored or retrieved from that bucket. 
* A client points partner connector A to an existing bucket that partner connector B had already been pointed to. 
	* Partner connector B receives the attribution for all of the data that is stored and retrieved from the bucket. 


## DESIGN

### Connector

Each partner will have a registered id, (which we will refer to as the partner id) that will identify a partners connector on the Storj network.  When a user uploads data to a specified bucket through the connector,  the connector will include the partner id in the content of the GRPC request. Before an upload occurs, the uplink will communicate the partner id and bucket name with the tardigrade satellite, checking for a previous attribution. If no attribution is found on the specified bucket and the bucket is currently void of data, the satellite will attribute the partners id to that bucket within the metadata struct. Concurrently to updating the metadata struct the satelitte will add the necessary data to the Attribution table. 


### Database

The attribution table will consist of data that allows for ease of calculating total attribution for a partner. 

| Name            | Type          |
| --------------- | ------------- |
| project_id (pk) | uuid          |
| bucket_name(pk) | bytes         |
| partner_id      | uuid          |
| last_updated    | timestamp     |


### Reporting

When the total attribution needs to be calculated for a partner, the attribution service will need to find all buckets attributed to the partner, list all objects for each bucket, and calculate the total storage and egress bandwidth was used during the time period specified. This will be done on an ad hoc basis.
Example satellite cli usage:
```
satellite reports partner-attribution <partner ID> <start date inclusive> <end date exclusive>
```