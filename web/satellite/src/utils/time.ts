// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// toUnixTimestamp converts Date to unix timestamp
export function toUnixTimestamp(time :Date) : number {
    return Math.floor(time.getTime() / 1000);
}
