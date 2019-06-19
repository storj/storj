# Storage Node Graceful Exit Design Document


## Overview

When a Storage Node wants to leave the network but does not want to lose their escrow we need to have a mechanism for them to exit the network “gracefully”.

This process including the Storage nodes transferring their pieces to other nodes so that the satellite does not have to repair those pieces because of a node exiting abruptly. The process of a Storage Node exiting gracefully including that node requesting a list of Storage Nodes to send their pieces to and updating the satellite with what nodes are now storing those pieces. Graceful Exit for Storage Nodes is beneficially for both the Storage Node and satellite because the storage node receives their escrow and the satellite saves money from not having to repair files.


## Goals

- Give Storage Nodes a mechanism to leave the network while receiving their escrows, and reduce repair caused by node churn on satellites. 


## Non Goals

- Sending the storage nodes escrows
	- The sending of tokens will happen through our normal token payment process


## Scenarios (MVP)

- A storage node gracefully exits the entire network (all satellites)
- A storage node fails to complete the graceful exit process


## Scenarios (Non MVP)

- A storage node runs out of bandwidth during the graceful exit process
- A storage node wants to “partially” exit a satellite
- A storage node wants to gracefully exit one satellite
- A storage node wants to rejoin a satellite it previously exited

## Business Requirements/ Job Stories (MVP)

- When a Storage Node runs the graceful exit command I want them to be prompted with a confirmation message so that we can avoid storage nodes running the command accidentally.
- When a Storage Node gracefully exits the network I want all of the pieces they have been storing to be deleted from their hard drive so that their computers hard drive space is no longer used. 
- When a Storage node triggers graceful exit I want them to be omitted from the node selection process for uploads so they are no longer selected to store new pieces.
- When a Storage node triggers graceful exit I want thier allocated bandwidth on the network to be ignored so that they can complete their graceful exit as quickly as possible.
- When a Storage Node leaves the network I want the ability to run a report on the satellite to get information about what Storage Nodes exited so that I can pay them thier escrow amounts. 
	- Create a report on the satellite to display which nodes have exited gracefully during a specified timeframe. The report must include:
		- NodeID
		- Wallet address
		- The date the node joined the network
		- The date the node exited
		- GB Transferred (amount of data the node transferred during exiting)		
- When a Storage Node exits the network gracefully I want the satellite to have the ability to track how much egress they used for exiting so that we do not pay them for that bandwidth.
- When A Storage Node is in the process of gracefully exiting the network I want them to be selected for download requests so that they can continue to contribute to the overall network.
- When a Storage Node is in the process of gracefully exiting the network I want them to continue to be audited and thier uptime checked so we can ensure they are still "good" nodes.
- When a Storage Node wants to rejoin the network I want them to have that ability so that they do not need to generate a new node ID via POW, go through the node vetting process, and so they can utilize their repuration. 
- When a Storage Node rejoins the network I want the satellite to keep track of that so that we can start the escrow process for that node over. 

## Business Requirements/ Job Stories (Non MVP)

- When a Storage Node completes graceful exit for all satellites I want their Node to automatically shut down so that they do not have to worry about receiving any more data.
- When a Storage node no longer wants to store data for a satellite I want them to have the ability to run graceful exit for a specific satellite so that they do not lose their escrow on that satellite. 
- When a Storage Node wants to rejoin a satellite they previously exited gracefully I want them to have a simple command they can run so that the satellite is notified and they can start being selected for storage in the node selection process.
	- The node will restart the escrow process if they decide to rejoin a satellite. 
- When a Storage Node operator wants to reduce the amount of storage space they have allocated to the network I want them to have the ability to do a “partial graceful exit which would transfer some of the data they currently have onto other nodes so they have a way of reducing thier storage allocation without just deleting data and failing audits. 
- When a Storage node triggers graceful exit on a satellite but they do NOT have enough allocated bandwidth to send the data to other nodes I want the SNO to be prompted about what action they would like to take. 
	- Continue the graceful exit and exceed the bandwidth allocation the SNO originally setup
	- Wait until the SNO has enough available bandwidth
	- Cancel the graceful exit 

## Design Overview

- Receiving storage nodes should be uploaded to as if they were receiving any other upload (orders, etc)
