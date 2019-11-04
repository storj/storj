# Zombie Segments Cleaner

## Abstract

This document describes design for cleaning segments that are not accessible with standard operations like listing.

## Background

Currently with a multisegment object, we have several places where we can leave unfinished or broken objects:
* if upload process will be stopped, because of an error or canceling, when one or more segments are already uploaded we can become zombie segments.
* if delete process will be stopped, because of an error or canceling, when one or more segments are already deleted we can have object with incomplete number of segments.
* in the past we had a bug where error during deleting segments from nodes was interrupting deleting segment from satellite, and we have segments available on satellite but not on storage node.

We need a system for identifying objects with missing segments, and zombie segments that are not listed as part of objects, that cleans up those segments.

In the long term, the metainfo refactor will fix this. We need a short term solution.

## Design


## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Implementation

[A description of the steps in the implementation.]

## Open issues (if applicable)