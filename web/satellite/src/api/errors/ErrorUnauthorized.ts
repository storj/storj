// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorUnauthorized is a custom error type for performing unauthorized operations.
 */
export class ErrorUnauthorized extends Error {
    public readonly status = 401;
    public requestID: string | null;

    public constructor(message = 'Authorization required', requestID: string | null) {
        super(message);
        this.requestID = requestID;
    }
}
