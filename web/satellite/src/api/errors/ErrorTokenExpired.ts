// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorTokenExpired is a custom error type for indicating that token links have expired.
 */
export class ErrorTokenExpired extends Error {
    public constructor(message = 'Token link expired') {
        super(message);
    }
}
