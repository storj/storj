// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO: functions should be moved to related business logic layer
/**
 * Returns held percentage depends on number of months that node is online.
 * @param startedAt date since node is online.
 */
export function getHeldPercentage(startedAt: Date): number {
    const monthsOnline = getMonthsBeforeNow(startedAt);

    switch (true) {
    case monthsOnline < 4:
        return 75;
    case monthsOnline < 7:
        return 50;
    case monthsOnline < 10:
        return 25;
    default:
        return 0;
    }
}

/**
 * Returns number of months passes till now.
 * @param startedAt date since node is online.
 */
export function getMonthsBeforeNow(startedAt: Date): number {
    const now = new Date();
    const yearsDiff =  now.getUTCFullYear() - startedAt.getUTCFullYear();

    return (yearsDiff * 12) + (now.getUTCMonth() - startedAt.getUTCMonth()) + 1;
}

export function centsToDollars(cents: number): string {
    return `$${(cents / 100).toFixed(2)}`;
}
