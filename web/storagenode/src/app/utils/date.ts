// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * returns difference between two dates in minutes
 * @param d1 - holds first date
 * @param d2 - holds second date
 */
export function datesDiffInMinutes(d1: Date, d2: Date): number {
    const diff = d1.getTime() - d2.getTime();

    return Math.floor(diff / 1000 / 60);
}

/**
 * returns difference between two dates in hours and minutes
 * @param d1 - holds first date
 * @param d2 - holds second date
 */
export function datesDiffInHoursAndMinutes(d1: Date, d2: Date): string {
    const secondsInHour = 3600;
    const diff = d1.getTime() - d2.getTime();

    if (Math.floor(diff / 1000) > secondsInHour) {
        const hours: string = Math.floor(diff / 1000 / secondsInHour) + 'h';
        const minutes: string = Math.floor((diff / 1000 % secondsInHour) / 60) + 'm';

        return hours + minutes;
    }

    return Math.floor(diff / 1000 / 60) + 'm';
}
