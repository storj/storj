# Storage Node Graceful Exit Design Document


## OVERVIEW

When a Storage Node wants to leave the network but does not want to lose their escrow we need to have a mechanism for them to exit the network “gracefully”.

This process including the Storage nodes transferring their pieces to other nodes so that the satellite does not have to repair those pieces because of a node exiting abruptly. The process of a Storage Node exiting gracefully including that node requesting a list of Storage Nodes to send their pieces to and updating the satellite with what nodes are now storing those pieces. Graceful Exit for Storage Nodes is beneficially for both the Storage Node and satellite because the storage node receives their escrow and the satellite saves money from not having to repair files.


## GOALS

- Give Storage Nodes a mechanism to leave the network while receiving their escrows, and reduce repair caused by node churn on satellites. 


## NON GOALS
- Sending the storage nodes escrows
	- The sending of tokens will happen through our normal token payment process


## SCENARIOS

- A storage node successfully completes a graceful exit
- A storage node cancels the graceful exit process
- A storage node shuts down during the graceful exit process
	- Power loss or reboot 
- A storage node runs out of bandwidth during the graceful exit process
- A storage node wants to “partially” exit a satellite
- A storage node wants to rejoin a satellite it previously exited


## Business Requirements/ Job Stories

- When a Storage Node runs the graceful exit command I want them to be prompted with a confirmation message so that we can avoid storage nodes running the command accidentally
	- Something like “Are you sure you want to run graceful exit? Graceful exit sends the 1.2 TB of data you are storing to other storage nodes so you will no longer be paid for storing this data. This process can take several hours to complete please your storage node running until it is completed.”
	- We need to specify the amount of data the Storage node will be sending to other nodes. 
- When a Storage Node completes graceful exit for all satellites I want their Node to automatically shut down so that they do not have to worry about receiving any more data.
- When a Storage Node completes graceful exit I want all of the data they have been storing to be deleted so that their computer is no longer storing any pieces. 
- When a Storage node no longer wants to store data for a satellite I want them to have the ability to run graceful exit for a specific satellite so that they do not lose their escrow on that satellite. 
	- Add the ability to run graceful exit for a specific satellite
	- The satellite must mark the storage node as ‘exited’ and the SN should update its satellite whitelist so that it rejects any future requests from that satellite.
- When a Storage Node operator wants to reduce the amount of storage space they have allocated to the network I want them to have the ability to do a “partial graceful exit which would transfer some of the data they currently have onto other nodes so they have a way of reducing thier storage allocation without just deleting data and failing audits. 
- When a Storage node triggers graceful exit for a satellite I want that node to be omitted from the node selection process so they are no longer selected to store data on the network
- When a Storage node triggers graceful exit on a satellite but they do NOT have enough allocated bandwidth to send the data to other nodes I want the SNO to be prompted about what action they would like to take. 
	- Continue the graceful exit and exceed the bandwidth allocation the SNO originally setup
	- Wait until the SNO has enough available bandwidth
	- Cancel the graceful exit 
- When a Storage Node wants to rejoin a satellite they previously exited gracefully I want them to have a simple command they can run so that the satellite is notified and they can start being selected for storage in the node selection process.
	- The node will restart the escrow process if they decide to rejoin a satellite. 
- Create a report on the satellite to display which nodes have exited gracefully during a specified timeframe
	- The report must include
		- NodeID
		- Wallet address
		- The date the node joined the network
		- The date the node exited
		- GB Transferred (amount of data the node transferred during exiting)
- When a node exits the network gracefully I want the satellite to have the ability to track what egress they used for exiting so that we do not pay them for that bandwidth. 


## DESIGN OVERVIEW

- Receiving storage nodes should be uploaded to as if they were receiving any other upload (orders, etc)
