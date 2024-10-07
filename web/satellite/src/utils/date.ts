// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Returns a Date object that is a specified number of days in the future.
 *
 * @param days - The number of days to add to the current date.
 * @returns A Date object representing the future date.
 */
export function dateAfterDays(days: number): Date {
    const laterDate = new Date();
    laterDate.setDate(new Date().getDate() + days);
    return laterDate;
}