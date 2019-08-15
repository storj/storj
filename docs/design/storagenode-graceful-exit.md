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
	- The Storage Node keep a separate and detailed metric of network bandwidth used to serve the data to clients vs bandwidth used for graceful exit. The Storage Node shall NOT get paid for bandwidth used for graceful exit but shall get paid for the bandwidth used to serve data to clients(downloads, audit egress)
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
- Update DBX - Add fields to `nodes` table
  ```
    model nodes (
		...
		field exit_loop_count           int
		field exit_initiated_at         timestamp ( updateable )
		field exit_completed_at         timestamp ( updateable )

	}
  ```
- Add GetExitingNodeIds method to overlaycache. Returns node IDs where `exit_initiated_at` is not null and `exit_completed_at` is null.
- Add GetExitedNodeIds method to overlaycache. Returns node IDs where `exit_initiated_at` is not null and `exit_completed_at` is not null. 
- Update DBX - Add table exit_pieceinfos
   ```
	model exit_pieceinfo (
		key node_id path

		field node_id           blob
		field path              blob
		field piece_num          blob
		field durability_ratio  float64
		field queued_at         timestamp ( autoinsert )
		field requested_at      timestamp ( updateable )
		field failed_at         timestamp ( updateable )
		field completed_at      timestamp ( updateable )
	)
   ```
- Update `cache.FindStorageNodesWithPreferences` to ignore exiting nodes for uploads and repairs.
- Add `PieceAction_PUT_EXIT` to orders protobuf. This is used to differentiate exiting bandwidth from other bandwidth usage.
- Create GracefulExit service
  - Add MetainfoObserver loop
    - The service queries the `nodes` to get a list of exiting node IDs where `exit_loop_count` <= `requiredMetainfoLoops`, then
      - Joins the metainfo loop `observers` using the exiting node IDs and looks for segments that contain remote pieces that the exiting node stores. It adds `node_id`, `path`, `piece_num`, `durability_ratio`, and `queued_at` in `exit_pieceinfo` if it does not exist.
	  - Join returns after one full metainfo loop, or on err. On success, the service should increment `nodes.exit_loop_count` for the nodeIDs that were processed in the full loop.
    - Execution intervals and requiredMetainfoLoops should be configurable
  - Add CheckStatus loop
    - Queries `exit_pieceinfo` grouping by node ID and counts completed, and incomplete.  If all records are complete and `nodes.exit_loop_count` <= `requiredMetainfoLoops`
      - Set `nodes.exit_completed` to current time.
      - Remove all `exit_pieceinfos` for the completed node ID
    - Execution intervals should be configurable
- Create GracefulExit endpoint
  - Endpoints should be secured using the peer Identity provided in context
  - InitiateExit initiates the exit process by setting `nodes.exit_initiated_at` to current time
  - GetPutOrders
    - Queries `nodes` by `id` and `exit_completed`.
      - If completed it returns `PutOrdersResponse` with `exit_completed` set to true
      - Else it checks whether there are any records in `exit_pieceinfo` for the node ID. If not, it returns any empty `PutOrdersResponse`. This covers the case where the metainfo loop hasn't added records into `exit_pieceinfo` yet. // TODO: maybe we return a time when the node should check again.
      - Else it checks whether there are any records in `exit_pieceinfo` where `exit_pieceinfo.requested_at` is not null and `exit_pieceinfo.failed_at` is not null or `exit_pieceinfo.completed_at` is null
        - If records are found, it should create new order limits for the found records, and return for reprocessing
        - Else it returns a list of order limits for pieces to be moved to new nodes from  `exit_pieceinfo` ordered by `durability_ratio`. durability_ratio == num pieces / optimal number of pieces. lower values take presedence.
  - ProcessPutOrders is called by the storagenode to send processed order limits. ProcessPutOrders verifies the hashes of successful orders limits, updates `exit_pieceinfo.completed_at`, and updates metainfo with the new piece location. Failures update `exit_pieceinfo.failed_at`, for reprocessing.
  - ``` 
	service GracefulExit {
		// Called by the storagenode to initiate an exit request
		rpc InitiateExit(InitiateExitRequest) returns (InitiateExitResponse) {}

		// Called by the storagenode to get a batch of piece orders to be moved to new nodes. Batch size TBD
		rpc GetPutOrders(PutOrdersRequest) returns (PutOrdersResponse) {}
		
		// Called by the storagenode to commit successful piece orders, and report failures in batch
		rpc ProcessPutOrders(ProcessPutOrdersRequest) returns (ProcessPutOrdersResponse) {}
	}

	message InitiateExitRequest {
	}

	message InitiateExitResponse {
	}


	message PutOrder {
		bytes hashing_key
		AddressedOrderLimit addressed_order_limit 
	}

	message PutOrdersRequest {
	}

	message PutOrdersResponse {
		bool exit_completed
		PutOrder put_orders repeatable		
	}

	message CompletedPutOrder {
		AddressedOrderLimit addressed_order_limit
		bytes piece_hash
	}

	message FailedPutOrder {
		AddressedOrderLimit addressed_order_limit
		// TBD reason
	}

	message ProcessOrdersRequest {
		CompletedOrder completed repeatable
		FailedOrder failed repeatable
	}
	```
- Create gracefulexitreport command in satellite cli
  - Accepts 2 parameter: start date and end date
  - Generates a list of all completed exits with NodeID, Wallet address, Date joined, Date exited, GB Transferred (calculated using `PieceAction_PUT_EXIT` bandwidth actions), ordered by date exited descending.

#### Storagenode
- Add `gracefulexit` command to storagenode CLI
  - When executed, the user should be prompted with a numbered list of satellites (what would we display as a name?)
  - After selecting a satellite, the user should be prompted for a confirmation
  - Once confirmed, the command should call the `GracefulExit.Initiate` endpoint for that satellite
  - Records starting disk usage in `gexit.exit_status`
- Add `gexit` DB implementation with `exit_status` and `exit_orders` tables
  - ```
	model exit_status (
		key satellite_id

		field satellite_id           blob not null
		field initiated_at           timestamp ( autoinsert ) not null
		field completed_at           timestamp ( updateable )
		field starting_disk_usage    int64 not null
		field bytes_deleted          int64
	)

	model exit_orders {
		key satellite_id piece_id

		field satellite_id           blob not null
		field piece_id               blob not null
		field piece_hash			 blob not null
		field order_limit            blob not null
		field failed                 bool
		field created_at             timestamp ( autoinsert ) not null
	}
	```
- Update bandwidth usage monitors to ignore `PieceAction_PUT_EXIT` bandwidth actions
- Add GracefulExit service
  - Checks for any records in `completed_exit_orders` that were processed, but not commited to the satellite.  If records exist, it should send a new  `ProcessOrdersRequest` to the satellite and remove the records in `exit_orders` on success.
  - Calls satellite `GracefulExit.GetPutOrders` and iterates over the orders in batchs
    - If `PutOrdersResponse.exit_completed` is true, then the process should update `exit_status.completed_at` and stop processing exit orders for this satellite
    - Else if `GracefulExit.GetPutOrders` is empty and `PutOrdersResponse.exit_completed` is false, the process should skip processing exit orders for this satellite for a specified time. TODO: hour? day?
    - Else
      - Pushes the pieces to the storage node identified in the order using piecestore and persists order limit and piece hash or failure to `exit_orders`. This is used to track processed orders that have not yet committed to the satellite.
      - On success, add order with the piece hash response to `ProcessOrdersRequest.completed`
      - On failure, the order should be added to `ProcessOrdersRequest.failed` 
    - Sends `ProcessOrdersRequest` using the satellite `ProcessOrders` endpoint
      - On success...
        - Removes the successful orders from `exit_orders`
        - Updates `bytes_deleted` with the number of bytes deleted
        - Deletes the pieces that were successfully moved
      - On failure (ex. satellite is unavailable), successful orders stored in `exit_orders` should be reprocessed on the next iteration
  - Execution intervals and batch sizes should be configurable

## Open Questions

- What happens if we are doing a repair and a graceful exit on the same segment?
	- Option 1. Leave the transferred piece as garbage on the new node. The GC process will take care of it.
	- Option 2. Once the exiting node sends the proof of transfer to the satellite, the satellite can check if the segment of this piece is still in the pointer db. If it was deleted, the satellite can send a delete request to the new node.
- What happens if a piece is deleted when the piece is in the process of being transferred from the exiting node to other node in the network?
