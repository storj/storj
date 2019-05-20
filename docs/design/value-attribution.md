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
Each partner will have a registered uuid that will identify a partners connector on the Storj network. Each connector will implement the S3 client interface to allow operations through the connector.  When a user uploads data to a specified bucket through the connector,  the connector will include the partner uuid in the context of the request. Before an upload occurs, the uplink will communicate the uuid and bucket name with the tardigrade satellite, checking for a previous attribution. If no attribution is found on the specified bucket and the bucket is currently void of data, the satellite will attribute the partners uuid to that bucket within the metadata struct.

After the connector has confirmed the partners uuid was successfully attributed to a bucket, the connector will notify a tardigrade satellite that the attribution was confirmed. The satellite will then look up the bucket, verify that partner has the attribution and add the necessary data to the Attribution table. 

### Database
The attribution table will consist of data that allows for ease of calculating total attribution for a partner. 

| Name  | Type |
| ------------- | ------------- |
| bucket_id (pk) | uuid  | 
| user_id  | uuid  |
| partner_id  | uuid  |
| total_data | integer  |
| last_updated | timestamp  |

### Reporting
When the total attribution needs to be calculated for a partner, the attribution service will need to find all buckets attributed to the partner, list all objects for each bucket and tally the total storage used. This can either be done on an ad hoc basis or a recurring interval.  After a calculation has been tallied, the updated total storage will be added to the attribution table.

## IMPLEMENTATION MILESTONES


