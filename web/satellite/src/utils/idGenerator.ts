// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Custom string id generator
export function getId(): string {
    return '_' + Math.random().toString(36).substr(2, 9);
}
