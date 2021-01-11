// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Size class contains currency related functionality such as convertation.
 */
export class Currency {
    /**
     * dollarsFromCents converts cents to dollars with prefix.
     * @param cents count
     */
    public static dollarsFromCents(cents: number): string {
        return `$${(cents / 100).toFixed(2)}`;
    }
}
