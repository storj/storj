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
- When a Storage node no longer wants to store data for a satellite, it shall have the ability to run graceful exit for a specific satellite so that they do not lose their escrow on that satellite. 
- When a Storage Node runs the graceful exit command, it shall be prompted with a confirmation message so that it can avoid running the command accidentally.
- When a storage node triggers the graceful exits, the process cannot be canceled. The storage node shall lead it to successful completion. Should the storagenode application, ungracefully terminated, upon restarting the storagnenode application, it shall resume the graceful exit process.
- When a Storage node triggers graceful exit, the satellite shall omit it from selecting the node for any future uploads and shall be eliminated from storing new pieces.
- When a Storage node triggers graceful exit, it shall ignore the allocated network bandwidth, so that it shall complete their graceful exit as quickly as possible.

### During exit
- When a Storage Node is in the process of gracefully exiting the network, it shall continue to participate in the requested audits and uptimes checks.
- When a Storage Node is in the process of gracefully exiting the network, it shall continue to honor to download requests.
	- The Storage Node keep a separate and detailed metric of network bandwidth used to serve the data to clients vs bandwidth used for graceful exit. The Storage Node shall NOT get paid for bandwidth used for graceful exit but shall get paid for the bandwidth used to serve data to clients(downloads, audits, uptime checks etc...)
- When a Storage Node is gracefully exiting the network, it shall delete the piece from its storage after it is successfully transferred to other peer Storage Nodes as directed by the Satellite. The Satellite shall support the mechanism to verify that the transferred piece is correct and complete.
- When a Storage Node is exiting the network gracefully, it shall keep a detailed record of the network bandwidth used for the purpose of Graceful Exit and shall provide the metrics information to satellite upon request. 
- The Satellite shall have a mechanism in place to make sure that the Storage Node reporting is correct. This shall eliminate Storage Node from reporting incorrect metrics interms of bandwidth usage. (TBD, is this needed on the satellite side??)

### When Exited 
- When a Storage Node left the network, the satellite shall have the capabilty to run a report to get information exited and/or exiting Storage Nodes, so that it shall pay them their escrow amounts accordingly. The report shall support the number of Storage Nodes left in a configurable specified time frame and shall include other information as shown below (TBD):
		- NodeID
		- Wallet address
		- The date the node joined the network
		- The date the node exited
		- GB Transferred (amount of data the node transferred during exiting)	
- When a Storage Node exits ungracefully, the satellite shall keep track of this and shall subject the Storage Node to be DQed and their escrow payment shall be denied. The factors that shall affect the escrow payments are 
	- the Storage Node tranferring incorrect data to other nodes
	- the Storage Node not transferring complete pieces it holds
- The Satellite shall keep track of the graceful exit status of the Storage Node and shall inform upon its completion. 
- The Storage Node upon receving the successful completion of graceful exit status from Satellite, it shall then delete any data it is holding. 


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

## Implementation (MVP)

#### Satellite
- Update DBX - Add updateable `exit_initiated_dt` and `exit_completed_dt` timestamps to nodes table with indexes
  - Add GetExitingNodeIds method to overlaycache. Returns nodes IDs where `exit_initiated_dt` is not null and `exit_completed_dt` is null.
- Create GracefulExit endpoint
  - Endpoints should be secured using the peer Identity provided in context
  - Initiates the exit by setting `nodes.exit_initiated` to current time
  - ``` 
	service GracefulExit {
		rpc Initiate(InitiateRequest) returns (InitiateResponse) {}
	}

	message InitiateRequest {
	}

	message InitiateResponse {
	}
	```
- Update DBX - Add table exit_pieceinfos
-  ```
	model exit_pieceinfo (
		key node_id path

		field node_id           blob
		field path              blob
		field peice_info        blob
		field durability_ratio  float64
		field queued_dt         timestamp ( autoinsert )
		field sent_dt           timestamp ( updateable )
		field completed_dt      timestamp ( updateable )
	)
   ```
- Add `PieceAction` field to, `cache.FindStorageNodesRequest`. Update `cache.FindStorageNodesWithPreferences` to ignore exiting nodes for uploads and repairs.
- Update Repairer service
  - Modify `checker` to check segments for pieces associated with a storage node that is exiting. Add to `exit_pieceinfo` table if matches criteria.
- Add `PieceAction_PUT_EXIT` to orders protobuf. This is used to differentiate exiting bandwidth from other bandwidth usage.
- Create GracefulExit service
  - SendOrders loop
    - Iterates over `exit_pieceinfo`, creates signed order limits with action type `PieceAction_PUT_EXIT`
    - Batches orders and sends them to the exiting storagenode `GracefulExit.ProcessOrders` endpoint
    - Execution intervals and batch sizes should be configurable
  - CheckStatus loop
    - Queries `exit_order` grouping by node ID where all records are marked completed.  The MAX(completed_dt) should be used to determine if the last order was completed within a reasonable (???) threshold to ensure the repairer/checker was able to make enough passes to capture all pieces for the exiting node.
    - Execution intervals should be configurable
- Create gracefulexitreport command in satellite cli
  - Accepts 2 parameter: start date and end date
  - Generates a list of all completed exits with NodeID, Wallet address, Date joined, Date exited, GB Transferred (calculated using `PieceAction_PUT_EXIT` bandwidth actions), ordered by date exited descending.

#### Storagenode
- Add `gracefulexit` command to storagenode CLI
  - When executed, the user should be prompted with a numbered list of satellites (what would we display as a name?)
  - After selecting a satellite, the user should be prompted for a confirmation
  - Once confirmed, the command should call the `GracefulExit.Initiate` endpoint for that satellite
  - Records starting disk usage in `gexit.exit_status`
- Add `gexit` DB implementation with `exit_order`  and `exit_order` tables
  - ```
	model exit_order (
		key satellite_id serial_number

		field satellite_id            blob not null
		field serial_number           blob not null
		field order_limit_serialized  blob not null
		filed order_limit_expiration  timestamp not null
		field completed_dt            timestamp ( updateable )
	)

	model exit_status (
		key satellite_id

		field satellite_id           blob not null
		field initiated_dt           timestamp ( autoinsert ) not null
		field completed_dt           timestamp ( updateable )
		field starting_disk_usage    int64 not null
		field bytes_deleted          int64
	)	
	```
- Add GracefulExit endpoint
  - Endpoint used by satellites to tell the exiting storage node what pieces to process

  - ``` 
	service GracefulExit {
		// Called by the satellite to batch piece orders to be moved to new nodes
		rpc ProcessOrders(OrderRequest) returns (OrderResponse) {}
		// Called by the satellite to notify the storagenode that the exit is complete for this satellite
		rpc Completed(CompletedRequest) returns (CompletedResponse)
		// Called by the satellite to get exit status information
		rpc Status(StatusRequest) returns (StatusResponse) {}
	}

	message Order {
		bytes hashing_key
		AddressedOrderLimit addressed_order_limit 
	}

	message OrderRequest {
		Order orders repeatable
	}

	message OrderResponse {
	}

    message CompletedRequest {
		google.protobuf.Timestamp completed_dt
	}

    message CompletedResponse {
	}

	message StatusRequest {
	}

	message StatusResponse {
        byte satellite_id
        google.protobuf.Timestamp initiated_dt
        google.protobuf.Timestamp completed_dt
        int64 starting_disk_usage
        int64 bytes_deleted		
	}
	```
- Update bandwidth usage monitors to ignore `PieceAction_PUT_EXIT` bandwidth actions
- Add GracefulExit service
  - Iterates over `exit_order` where `completed_dt` is null
    - Pushes the pieces to the storage node identified in the order using `ecclient`
    - Sends the signed new node response to the satellite via a new `CommitPiece` method (uses `metainfo.UpdatePieces`).
    - Updates `bytes_deleted` with the number of bytes deleted and sets `completed_dt` to current time
  - Execution intervals and batch sizes should be configurable

## Open Questions

- What happens if we are doing a repair and a graceful exit on the same segment?
	- Option 1. Leave the transferred piece as garbage on the new node. The GC process will take care of it.
	- Option 2. Once the exiting node sends the proof of transfer to the satellite, the satellite can check if the segment of this piece is still in the pointer db. If it was deleted, the satellite can send a delete request to the new node.
- What happens if a piece is deleted when the piece is in the process of being transferred from the exiting node to other node in the network?
