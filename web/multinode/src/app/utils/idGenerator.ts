// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

export function getId(): string {
    return '_' + Math.random().toString(36).substr(2, 9);
}