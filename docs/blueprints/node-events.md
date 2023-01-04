# Node Events

## Abstract

This document presents a design for identifying and processing node events for the purpose of notifying node operators. 

## Background

There are certain events we want to notify node operators about: their node goes offline, it's disqualified, etc. Currently, the way we notify operators is by periodically querying data for all nodes from the nodes DB and sending it to a 3rd party service which sends emails based on changes from the last data set it received: https://github.com/storj/dataflow/blob/main/flows/nodes_status/main.go. However, we've had problems with notifications being quite delayed with this method. Instead, the satellite with determine when these node events occur and trigger notifications.

### Node Events

Node Offline
    - when the nodes table column `last_contact_success` is more than some configured amount of time in the past.

Node Online 
    - when the nodes table column `last_contact_success` was previously more than some configured amount of time in the past, but is no longer.

Node Suspended (errors)
    - when the nodes table column `unknown_audit_suspended_at` changes from NULL to not NULL.

Node Unsuspended (errors)
    - when the nodes table column `unknown_audit_suspended_at` changes from not NULL to NULL.

Node Suspended (offline)
    - when the nodes table column `offline_suspended` changes from NULL to not NULL.

Node Unsuspended (offline)
    - when the nodes table column `offline_suspended` changes from not NULL to NULL.

Node Disqualified
    - when the nodes table column `disqualified` changes from NULL to not NULL.

Node Software Update
    - when a node's version is below some configured value.


## Design

There are two main components to this design: the node events DB and the node events chore.

At the time of one of the above events, we will insert it into a new DB called NodeEvents. 

```
type NodeEvent struct {
    id             bytes 
    node_id        bytes
    event          int
    email          string
    created_at     time.Time
    last_attempted time.Time
    email_sent     bool
}
```

The node events chore runs periodically and checks if there are any events in the DB for which to send notifications. If the least recently attempted, oldest event in the DB has existed for some minimum amount of time, it and all other events with the same email and event type are selected. The data is then passed to the Notifier. The Notifier is an interface to allow different implementations of notifying the storage node operator.

```
type Notifier interface {
    Notify(eventsData []NodeEvent) error
}
```

If the notification is successful, `email_sent` is set to true. If it is not, `last_attempted` is updated. `last_attempted` is used to avoid getting stuck retrying the same items over and over if something goes wrong. The chore then goes through this process again. If no events eligible for notification are found, the chore sleeps for some time and checks again.

For the "Node Online", "Node Software Update", "Node Suspended", "Node Unsuspended", and "Node Disqualified" emails, there is an event which occurs in the code where we can simply insert the event into the node events DB. For "Node Offline" we will add a chore to check the nodes table periodically for nodes that have not been seen in some time and insert them into the node events DB.

#### Node Online

Nodes initiate contact with the satellite every hour during a process called "check in". This ensures that the satellite has up-to-date information about the node and that it can be reached. On a successful check in the satellite reads the node's current information in the nodes table to see if an update is necessary. Here, we can check the node's current `last_contact_success`. If the old value of `last_contact_success` means the node is considered offline, insert "Node Online" into node events since it just checked in successfully.

#### Node Software Update

During the previously mentioned node check in process, one piece of information the node sends is its version. We can check the node's version here against a minimum version config to insert a "Node Software Update" event into node events. To avoid notifying the node every time it checks in, a new column will be added to the nodes table indicating the last time one of these emails was sent. With this we can make sure to wait a certain amount of time between emails.
  
#### Node Suspended (Offline, Errors), Unsuspended (Offline, Errors), Disqualified (some)

"Offline" and "Unknown Error" Suspensions occur in the same place: the satellite's `reputation` package. When an audit GET or a repair GET request occurs, how the node responds to the request (success, failure, offline, unknown error) is reported to the reputation service which then determines if the node is suspended/unsuspended or disqualified (disqualification also happens in other places, see the next section). This is where we can insert suspension/unsuspension events and some disqualification events into the node events DB. 

#### Node Disqualified

In addition to disqualification by the reputation service, disqualifications happen in two other places: the stray nodes chore and graceful exit. We can insert code in these places to insert DQ events into node events DB.

#### Node Offline

A node becoming considered offline is not an event that occurs in the code. Rather, it is the lack of an event, namely the node successfully checking in with the satellite. Therefore, we need a chore on the satellite to look for nodes where `last_contact_success` < X every hour so we can insert the event into the node events DB. In addition to `last_contact_success`, we need to look at another variable, `last_offline_email`, in order to avoid sending node operators a "Node Offline" notification every hour. `last_offline_email` will be a new column in the nodes table. When the chore inserts a "Node Offline" into node events, it will update this column for that node. This column will be cleared during a node's successful satellite check in. 

## Rationale

- Identifying node events in the satellite code rather than reading the nodes table and looking for changes:

    1. Some events might not be visible in the nodes table. We have more flexibility in what events we can notify for.
    2. Don't need to periodically run a giant DB query to get all status information for every single node. Just identify events at the time of occurrence.
    3. Notifications can potentially be faster given that they are identified at the time of occurence.
    4. Node events built into satellite. Community satellites just need to implement the Notifier interface.

- Sending node events to a central location rather than notifying immediately at the time of event:

    1. Condense multiple node events into a single notification

        Condensing node event notifications where possible is more convenient for the node operator, and can help avoid our emails being marked as spam. There are a couple situations where we can do this:

        First, we can handle duplicate events if they occur within a certain timeframe. For example, the satellite runs multiple instances of the reputation service. To reduce DB load, each reputation service has a cache. If a node on the verge of a state change is selected for multiple audits/repairs in a short period, each reputation cache that determines the node's state has changed would insert a node event into the DB and the node would get multiple notifications for the same thing. The node events chore looks for an entry which has existed for some amount of time before selecting it and all other entries of the same event type and email address in order to condense them into a single notification.

        Second, we can condense notifications for multinode operators if the events occur within a certain timeframe. For example, an operator running multiple nodes on their LAN experiences a network outage. Rather than receiving an email for each node, they would receive one email listing each node that is offline. Of course, if the node events are not clustered together then individual notifications would be sent. This depends on how long we would like the node events chore to wait before selecting events to process.

    2. Retry failed notifications more gracefully

        Imagine the repair worker sends node event notifications to a 3rd party endpoint directly. This 3rd party service goes down. The repair worker could be held up retrying the notification, or it could drop it and move on. Alternatively, it could send the notification to a central location to be retried later in the event of failures, but then we lose the above benefits.

## Open issues

- If the satellite goes down for more than four hours, when it comes back up all nodes would be notified that they are offline.
- If node events cannot be processed fast enough, we may need to implement concurrent processing like audit and repair workers.

## Implementation

- add `last_offline_email` and `last_version_email` to nodes table
- create node events DB
- create node events chore
- implement a Notifier to perform the notifications
- implement offline nodes chore
- insert events into node events DB in the locations described in the document
