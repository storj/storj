# Storage Node Graceful Exit Design Document


## Overview

When a Storage Node wants to leave the network but does not want to lose their escrow we need to have a mechanism for them to exit the network “gracefully”.

This process including the Storage nodes transferring their pieces to other nodes so that the satellite does not have to repair those pieces because of a node exiting abruptly. The process of a Storage Node exiting gracefully including that node requesting a list of Storage Nodes to send their pieces to and updating the satellite with what nodes are now storing those pieces. Graceful Exit for Storage Nodes is beneficially for both the Storage Node and satellite because the storage node receives their escrow and the satellite saves money from not having to repair files.


## Goals

- Give Storage Nodes a mechanism to leave the network while receiving their escrows, and reduce repair caused by node churn on satellites. 


## Non Goals

- Sending the storage nodes escrows.
	- The sending of tokens will happen through our normal token payment process.


## Scenarios (MVP)

- A storage node gracefully exits a satellite or all satellites.
- A storage node fails to complete the graceful exit process.


## Scenarios (Non MVP)

- A storage node runs out of bandwidth during the graceful exit process.
- A storage node wants to “partially” exit a satellite.
- A storage node wants to rejoin a satellite it previously exited.


## Business Requirements/ Job Stories (MVP)


### On Trigger
- When a Storage node no longer wants to store data for a satellite I want them to have the ability to run graceful exit for a specific satellite so that they do not lose their escrow on that satellite. 
- When a Storage Node runs the graceful exit command I want them to be prompted with a confirmation message so that we can avoid storage nodes running the command accidentally.
- When a storage node triggers the graceful exits, the process cannot be canceled. The storage node must lead it to successful completion.
- When a Storage node triggers graceful exit I want them to be omitted from the node selection process for uploads so they are no longer selected to store new pieces.
- When a Storage node triggers graceful exit I want their allocated bandwidth on the network to be ignored so that they can complete their graceful exit as quickly as possible.

### During exit
- When a Storage Node is in the process of gracefully exiting the network I want them to continue to be audited and their uptime checked so we can ensure they are still "good" nodes.
- When A Storage Node is in the process of gracefully exiting the network I want them to be selected for download requests so that they can continue to contribute to the overall network.
	- we need to be able to distinguish the bandwidth used for serving up data to clients vs bandwidth used for graceful exit. the node will NOT get paid for bandwidth used for graceful exit but will get paid for the bandwidth used to serve data to clients.
- When a Storage Node is gracefully exiting the network I want all of the pieces they are storing to be deleted from their hard drive as they exit so that their computers hard drive space is no longer used.
- When a Storage Node is exiting the network gracefully I want the satellite to have the ability to track how much egress they used for exiting so that we do not pay them for that bandwidth.

### When Exited 
- When a Storage Node leaves the network I want the ability to run a report on the satellite to get information about what Storage Nodes exited so that I can pay them their escrow amounts. 
	- Create a report on the satellite to display which nodes have exited gracefully during a specified time frame. The report must include:
		- NodeID
		- Wallet address
		- The date the node joined the network
		- The date the node exited
		- GB Transferred (amount of data the node transferred during exiting)	
- When a node does not complete the graceful exit entirely I want the satellite to keep track of this so that they are subject to be DQed and their escrow kept
	- this includes the node sending bad data to other nodes
	- the node not transferring some pieces it holds
- When the satellite determines the exiting node has completed the process I want the node to be informed so that it can delete the garbage data it is holding. 


## Business Requirements/ Job Stories (Non MVP)

- When a Storage Node completes graceful exit for all satellites I want their Node to automatically shut down so that they do not have to worry about receiving any more data.
- When a Storage Node wants to rejoin a satellite they previously exited gracefully I want them to have a simple command they can run so that the satellite is notified and they can start being selected for storage in the node selection process.
	- The node will restart the escrow process if they decide to rejoin a satellite. 
- When a Storage Node operator wants to reduce the amount of storage space they have allocated to the network I want them to have the ability to do a “partial graceful exit which would transfer some of the data they currently have onto other nodes so they have a way of reducing their storage allocation without just deleting data and failing audits. 
	- In this situation the satellite will determine which pieces are removed from the node NOT the storage node.
- When a Storage node triggers graceful exit on a satellite but they do NOT have enough allocated bandwidth to send the data to other nodes I want the SNO to be prompted about what action they would like to take. 
	- Continue the graceful exit and exceed the bandwidth allocation the SNO originally setup
	- Wait until the SNO has enough available bandwidth
- When a Storage Node wants to rejoin the network I want them to have that ability so that they do not need to generate a new node ID via POW, go through the node vetting process, and so they can utilize their reputation. 
- When a Storage Node rejoins the network I want the satellite to keep track of that so that we can start the escrow process for that node over. 


## Design Overview

- The repair checker will be responsible for finding pieces that need to be transferred from a node who is gracefully exiting the network.
- The repair checker needs to add those pieces to another queue.
	- This queue is responsible for prioritizing the order of pieces the exiting node sends to other nodes.
		- pieces should be prioritized based on how many other pieces are still on the network. Pieces for files that are closer to needing to be repaired should be transferred off the exiting before pieces for files that are still considered healthy.
	- This queue is responsible for communicating with the exiting node and telling it what piece to send to what node
	- This should be a unique queue for each exiting node
- The satellite needs to validate the correctness of the data the exiting node transferred to the new node
	- The exiting node will send the new node the hash it has for the piece. (that hash contains the uplinks signature). The new node will sign it and send it back to the exiting node. The exiting node will send it to the satellite to prove the data was transferred to the new node. The satellite will check the uplinks signature and the new nodes signature on the hash it receives from the exiting node.
- We need to create a new operation for storage nodes transferring pieces to other storage nodes
	- Receiving storage nodes should be uploaded to as if they were receiving any other upload (orders, etc).
- We need to have the storage node keep track how its progress during graceful exit by calculating how much data it had when it started the graceful exit and keeping track of how much data it has deleted (data is deleted from the exiting node as it sends pieces to new nodes). This is an estimation since the storage node is likely holding more data than it will gracefully exit (garbage data). 


## Open Questions

- What happens if we are doing a repair and a graceful exit on the same segment?
	- Option 1. Leave the transferred piece as garbage on the new node. The GC process will take care of it.
	- Option 2. Once the exiting node sends the proof of transfer to the satellite, the satellite can check if the segment of this piece is still in the pointer db. If it was deleted, the satellite can send a delete request to the new node.
- What happens if a piece is deleted when the piece is in the process of being transferred from the exiting node to other node in the network?
