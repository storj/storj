// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ErrorBadRequest is a custom error type for performing invalid operations.
 */
 export class ErrorBadRequest extends Error {
    public constructor(message: string = 'bad request') {
        super(message);
    }
}
