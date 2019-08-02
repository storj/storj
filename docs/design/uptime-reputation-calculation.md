# Uptime reputation caculation

## Abstract

Haveing a fair, simple and understandable way to calculate the uptime of _SNs_ is crucial for not unfairly disqualifying them, makes clear how _SNOs_ should administrate and maintain their _SN_ for being align with the network needs and have a clear proof of disqualification for _SN_ getting disqualified due to too many uptime checks failures.

## Background

Currently our disqualification system is described in the [disqualification design doc](disqualification.md) which is based in the cacluation of different _SN_ reputation scores as described in [node-selection design doc](node-selection.md).

This design doc is focused in how to design and implement a fair, simple and undestandble uptime reputation score calcuation after our initial approach din't behave as expected causing the undisqualication of several nodes without having a clear way to proof it and not being able o show that such disqualification was fair.