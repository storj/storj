# SNOBoard

## Abstract

The SNOBoard is a user interface for Storage Node Operators to get more information about their node.

Wireframes: https://www.figma.com/file/IlY5UNF94rEpCLGR6BeaOhXX/Local-Storage-Node-Dashboard-Low-Fidelity-Mock?node-id=7%3A520 

## Goals

Give Storage Node Operatores a user interface to get insight on:

* Node(s) performance per satellite
* Storage space
* Bandwidth
* Audit information
* Uptime information
* Current Status
* SN version currently running
* DQ’ed status
* Containment status

Handle notifications:

* Notifications center on the SNOBoard
* Email notification opt-in

Payout information:
* Expected payout per satellite

Configuration:
* Ability to adjust allocated egress and storage space
* Ability to gracefully exit a satellite or the network
* Ability to add trusted satellites.

## Business Requirements / Stories

When a SNO starts the CLI dashboard I want the URL for the SNOBoard to be printed on the screen so that the user can easily access the SNOBoard.

When my SN is connected to multiple satellites I want the ability to select a satellite on the SNOBoard so that I can drill down into information about my node as it pertains to that satellite.
* All satellites should be the default option
* This option should NOT show information about uptime or audit checks because those are specific to satellites
* The “All Satellites” option should only show information about allocated and used storage space/ bandwidth

When I want to know how much of the allocated storage space my SN has used on the network I want a simple graphic on the SNOBoard so that I can consume this information quickly

When I need to know how much of the allocated bandwidth my SN has used I want a simple graphic on the SNOBoard so that I can consume this information quickly

When I am looking at the bandwidth graphic I want to be able to distinguish how much bandwidth has been egress vs ingress so that I can determine if my internet plan is suitable for the type of traffic my node is getting. 

When a SNO is running an outdated version of SN software I want that information to be displayed on the SNOBoard so that they SNO can determine if they need to upgrade their software

When I need to know what wallet address my SN is configured with I want that information to be on the SNOBoard so that I can figure out what wallet I should be expecting my payout on

When a SNO starts the SNOBoard I want a set of links on the interface they can reference to get connected to the community or get more information about running their SN:
* Include a link to the community 
* Include a link to the support portal
* Include a link to the SNO documentation
* Include a link to the Aha Ideas portal
