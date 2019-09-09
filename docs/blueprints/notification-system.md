# Notification System (Foundation)

### Stefan Benten


## Definitions

*SN* - _Storagenode_

*SA* - _Satellite_

## Overview

Currently our network lacks the ability to let SA's inform the SN about issues, updates and other important messages/notifications.
With the Notification System the SA is able to send an email to the SN advertised email as send a message via simple Protobuf.

## Goals


* A generic library that can send emails and messages to the SN

* Real Time Updates / Information for the Operators in case of failed audits, uptime checks and future features like entering containment mode, being disqualified, etc.

* Improve Network Stability by helping our SN Operators to improve their nodes with more appropriate Feedback 

* Ability to send updates to specific groups of SN


## Non-Goals

* PopUp Messages on the SN Hosts

* No Push Notification System (getting updates to your mobile phone, etc.)

* Not preventing Notifications from every satellite currently, meaning, that you get an offline notification from every satellite you are connected to. (Possible way to mitigate this, is to first check via GRPC, if the node accepts connections)

## Scenarios


* The SA encounters a failed uptime check of/to SN.

	* It stores the updated metrics as usual in the database. 

	* Before going on, it send a notification to the SN about the change. 

* The SA encounters a passing uptime check to SN after having failed checks before. 

	* Same as above, but now sends an information that everything is fine again. 

* The SA encounters a failed audit of SN.

	* It stores the updated metrics as usual.

	* Sends a notification to the SN with the current status (rather that it entered Containment Mode, or failed the final audit)
	
	* In the later of above cases, it will also send the current ratio and its distance to the _disqualification bar_
	
* The SN software is outdated

	* The Satellite informs the SN about its outdated software and kindly reminds to update. This can be used to stage updates across the network once we have auto-updates in place.

## Design
### Functionality
The existing Service (mailservice) will be extended with templates for all possible notifications we want to send.

### New Service
A new service called NotificationService is added which relies on an existing configured mailservice.
It's API/function calls can be used throughout the satellites code base to add the corresponding hooks, where ever necessary.

```
type NotificationService struct {
	mailer *Mailservice
	cache  *Overlaycache
	
	config *Config
	Nodes  []*Status
}

// Config defines global settings for the amount of notifications that are sent out
type Config struct {
	// Defines maximum emails per Node per current Hour
	MaxHr int
	// Defines maximum emails per Node per current Day
	MaxDay int
	// Minimum Time between emails
	MinDelay time.Duration
}
// Status contains the per Node Information about the current notification quantities
type Status struct {
	// Counter to prevent over-sending per hour
	HrCounter  int
	// Counter to prevent over-sending per day
	DayCounter int
	// Timestamp of last email
	LastEmail  time.Time
}
```

### Rough workflow
```
func (notify *NotificationService) InformNodeEmail( nodeID string, message *mailservice.Message) (err error){
	//Get NodeInformation
	node, err := notify.cache.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	// Checking, that we did not send to many emails already
	allowed, err := compareStatusforAllowance(node.ID)
	
	err = notify.mailservice.SendRendered(ctx, node.email, message.Template...)
	if err != nil {
		return err
	}
	return nil
}
```

The same workflow is created for direct contact via a simple message protobuf to handle communication directly to the nodes log. Once the SNO Dashboard is up, we can make it an alert on there.
