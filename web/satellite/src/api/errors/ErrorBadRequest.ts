// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorBadRequest is a custom error type for performing invalid operations.
 */
export class ErrorBadRequest extends Error {
    public readonly status = 400;
    public requestID: string | null;

    public constructor(message = 'bad request', requestID: string | null) {
        super(message);
        this.requestID = requestID;
    }
}
