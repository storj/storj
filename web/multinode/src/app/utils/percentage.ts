// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Percentage class contains percentage related functionality.
 */
export class Percentage {
    /**
     * dollarsFromCents converts cents to dollars with prefix.
     * @param number float
     */
    public static fromFloat(number: number): string {
        return `${parseFloat(`${(number * 100).toFixed(1)}`)}%`;
    }
}
