// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Time holds methods to operate over timestamps.
 */
export class Time {
    /**
     * toUnixTimestamp converts Date to unix timestamp.
     * @param time
     */
    public static toUnixTimestamp(time: Date): number {
        return Math.floor(time.getTime() / 1000);
    }
}
