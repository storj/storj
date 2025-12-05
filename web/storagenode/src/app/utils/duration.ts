// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const millisecondsInSecond = 1000;
export const secondsInHour = 3600;
export const secondsInMinute = 60;
export const minutesInHour = 60;

/**
 * Duration provides the elapsed time between two date instants.
 */
export class Duration {
    public static difference(firstDate: Date, secondDate: Date): number {
        return Math.floor((firstDate.getTime() - secondDate.getTime()));
    }
}
