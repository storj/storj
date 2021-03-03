# Sparse Order Storage Rollout

## Problem Statement

We need to fix all of the underspend/doublespends as described in

  - https://storjlabs.atlassian.net/browse/SM-1187
    - Nodes can submit with the new endpoint and then the old endpoint to double spend
  - https://storjlabs.atlassian.net/browse/SM-1242
    - Orders may be placed in the queue, then submitted with the new endpoint, then processed leading to a double spend
  - https://storjlabs.atlassian.net/browse/SM-1241
    - Nodes may accidentally underspend by submitting some orders in a window with the old endpoint, upgrading, then being unable to submit the remainder of the window with the new endpoint

## Current Design

We have two endpoints: an old one and a new one. The behaviors are

### Old
  - Check for validity of the orders, removing invalid ones
  - Place all valid orders into the queue
  - A worker eventually consumes the queue and updates any existing rollup rows

Note that orders submitted through the old endpoint may be submitted multiple times without double spends because the queue worker handles that.

### New
  - Checks for validity of the orders, removing invalid ones
  - Ensures all orders are in the same window
  - Checks if existing rollup rows exist for that window
    - If the rows exist
      - Returns accepted if submitted orders match
      - Returns rejected if submitted orders don't match
    - Otherwise creates rows with sum of submitted orders

Additionally, once a node uses the new endpoint, any attempts from that node to use the old endpoint will be rejected.

## Proposed Design

The rollout will happen in three phases. In phase 1 the endpoints will

### Old
  - Remain unchanged from the current design

### New
  - Check for validity of the orders, removing invalid ones
  - Ensures all orders are in the same window
  - Place all valid orders into the queue
  - Create rollup rows with settled amount equal to 0 if they do not already exist
  - A worker eventually consumes the queue and updates existing rollup rows

After all (or enough) of the nodes are using the new endpoint, we switch to phase 2 where the endpoints will

### Old
  - Be removed

### New
  - Remain unchanged from phase 1

Once we have waited longer than 2 days (the order expiration time), we know that every valid order will be submitted with the new endpoint only. Then we move to phase 3 where the endpoints will

### Old
  - Remain removed

### New
  - Go back to the current design

We then wait for the queue to be fully drained, and then remove the worker and the queue tables.

## Discussion

The reason why this fixes double spends is because when orders are submitted with the new endpoint in phase 1, we create rollup rows with settled amount equal to 0. That means that when we move to phase 2, it will reject any attempts to submit to a window that you already submitted to.

The reason why this fixes under spends is because during phase 1, we allow the orders to be submitted multiple times and let the queue and worker handle the double submissions like it already does.

The reason why we have the middle phase 2 is so that a node cannot go directly from using the current design old endpoint to the current design new endpoint with valid orders. If that were allowed, then we would have the same double/under spends that we are trying to avoid.
