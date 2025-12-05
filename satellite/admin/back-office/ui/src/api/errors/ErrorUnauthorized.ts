// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorUnauthorized is a custom error type for performing unauthorized operations.
 */
export class ErrorUnauthorized extends Error {
    public constructor(message = 'Authorization required') {
        super(message);
    }
}
