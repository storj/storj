// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorTooManyRequests is a custom error type for performing 'too many requests' operations.
 */
export class ErrorTooManyRequests extends Error {
    public readonly status = 429;
    public requestID: string | null;

    public constructor(message = 'Too many requests', requestID: string | null) {
        super(message);
        this.requestID = requestID;
    }
}
